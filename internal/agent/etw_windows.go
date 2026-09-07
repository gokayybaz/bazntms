//go:build windows

// İnce ETW (Event Tracing for Windows) gerçek-zamanlı tüketicisi — Faz 20 S20.8.
//
// Saf Go: advapi32 syscall'ları + Windows SDK struct düzenleri elle sarılır
// (yeni bağımlılık yok, cgo yok, MIT-temiz). Yalnızca amd64 Windows hedeflenir
// (agent yayın matrisi windows/amd64). Struct boyutları checkLayout() ile
// çalışma anında doğrulanır — uyuşmazlıkta oturum açılmaz, seçici pcap'e düşer.
package agent

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32          = windows.NewLazySystemDLL("advapi32.dll")
	procStartTraceW   = advapi32.NewProc("StartTraceW")
	procEnableTrace2  = advapi32.NewProc("EnableTraceEx2")
	procControlTraceW = advapi32.NewProc("ControlTraceW")
	procOpenTraceW    = advapi32.NewProc("OpenTraceW")
	procProcessTrace  = advapi32.NewProc("ProcessTrace")
	procCloseTrace    = advapi32.NewProc("CloseTrace")
)

const (
	wnodeFlagTracedGUID        = 0x00020000
	eventTraceRealTimeMode     = 0x00000100
	procTraceModeRealTime      = 0x00000100
	procTraceModeEventRecord   = 0x10000000
	eventControlEnableProvider = 1
	eventTraceControlStop      = 1
	traceLevelVerbose          = 0xFF
	invalidTraceHandle         = ^uintptr(0)

	errAlreadyExists = 183
	errCancelled     = 1223
)

// rtLostEventGUID {6A399AE0-4BC6-4DE9-870B-3657F8947E7E} — gerçek-zamanlı
// oturumda düşen olayları bildiren sözde-sağlayıcı.
var rtLostEventGUID = windows.GUID{
	Data1: 0x6A399AE0, Data2: 0x4BC6, Data3: 0x4DE9,
	Data4: [8]byte{0x87, 0x0B, 0x36, 0x57, 0xF8, 0x94, 0x7E, 0x7E},
}

// ─── Windows SDK struct düzenleri (evntrace.h / evntcons.h, x64) ───

type wnodeHeader struct {
	BufferSize        uint32
	ProviderId        uint32
	HistoricalContext uint64
	TimeStamp         int64
	Guid              windows.GUID
	ClientContext     uint32
	Flags             uint32
}

type eventTraceProperties struct {
	Wnode               wnodeHeader
	BufferSize          uint32
	MinimumBuffers      uint32
	MaximumBuffers      uint32
	MaximumFileSize     uint32
	LogFileMode         uint32
	FlushTimer          uint32
	EnableFlags         uint32
	AgeLimit            int32
	NumberOfBuffers     uint32
	FreeBuffers         uint32
	EventsLost          uint32
	BuffersWritten      uint32
	LogBuffersLost      uint32
	RealTimeBuffersLost uint32
	LoggerThreadId      uintptr
	LogFileNameOffset   uint32
	LoggerNameOffset    uint32
}

type enableTraceParameters struct {
	Version          uint32
	EnableProperty   uint32
	ControlFlags     uint32
	SourceId         windows.GUID
	EnableFilterDesc uintptr
	FilterDescCount  uint32
}

type systemTime struct {
	Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16
}

type timeZoneInformation struct {
	Bias         int32
	StandardName [32]uint16
	StandardDate systemTime
	StandardBias int32
	DaylightName [32]uint16
	DaylightDate systemTime
	DaylightBias int32
}

type traceLogfileHeader struct {
	BufferSize         uint32
	Version            uint32
	ProviderVersion    uint32
	NumberOfProcessors uint32
	EndTime            int64
	TimerResolution    uint32
	MaximumFileSize    uint32
	LogFileMode        uint32
	BuffersWritten     uint32
	// C'de: union { GUID LogInstanceGuid; struct { ULONG StartBuffers,
	// PointerSize, EventsLost, CpuSpeedInMHz; } }. HER İKİ dal da 16 bayt.
	// LoggerName/LogFileName/TimeZone bu union'ın İÇİNDE değil, ARDINDAN gelen
	// ayrı alanlar (evntrace.h). Bu 16 baytı atlamak, eventTraceLogfileW
	// içindeki EventRecordCallback ofsetini 16 bayt kaydırır → callback hiç
	// çağrılmaz.
	LogInstanceGuid [16]byte
	LoggerName      uintptr
	LogFileName     uintptr
	TimeZone        timeZoneInformation
	BootTime        int64
	PerfFreq        int64
	StartTime       int64
	ReservedFlags   uint32
	BuffersLost     uint32
}

type eventTraceHeader struct {
	Size           uint16
	FieldTypeFlags uint16
	Version        uint32
	ThreadId       uint32
	ProcessId      uint32
	TimeStamp      int64
	Guid           windows.GUID
	KernelTime     uint32
	UserTime       uint32
}

type etwBufferContext struct {
	ProcessorNumber uint8
	Alignment       uint8
	LoggerId        uint16
}

type eventTrace struct {
	Header           eventTraceHeader
	InstanceId       uint32
	ParentInstanceId uint32
	ParentGuid       windows.GUID
	MofData          uintptr
	MofLength        uint32
	BufferContext    etwBufferContext
}

type eventTraceLogfileW struct {
	LogFileName         *uint16
	LoggerName          *uint16
	CurrentTime         int64
	BuffersRead         uint32
	ProcessTraceMode    uint32
	CurrentEvent        eventTrace
	LogfileHeader       traceLogfileHeader
	BufferCallback      uintptr
	BufferSize          uint32
	Filled              uint32
	EventsLost          uint32
	EventRecordCallback uintptr
	IsKernelTrace       uint32
	Context             uintptr
}

type eventDescriptor struct {
	Id      uint16
	Version uint8
	Channel uint8
	Level   uint8
	Opcode  uint8
	Task    uint16
	Keyword uint64
}

type eventHeader struct {
	Size            uint16
	HeaderType      uint16
	Flags           uint16
	EventProperty   uint16
	ThreadId        uint32
	ProcessId       uint32
	TimeStamp       int64
	ProviderId      windows.GUID
	EventDescriptor eventDescriptor
	KernelTime      uint32
	UserTime        uint32
	ActivityId      windows.GUID
}

type eventRecord struct {
	EventHeader       eventHeader
	BufferContext     etwBufferContext
	ExtendedDataCount uint16
	UserDataLength    uint16
	ExtendedData      unsafe.Pointer
	UserData          *byte // ETW buffer'ı — yalnızca callback süresince geçerli
	UserContext       unsafe.Pointer
}

// checkLayout, struct boyut/ofsetlerinin Windows/amd64 ABI ile uyuştuğunu
// doğrular. Uyuşmazlık → oturum açılmaz (seçici pcap'e düşer) — bellek
// bozulması yerine. Beklenen değerler evntrace.h'den elle + çalışan bir
// referans implementasyonla (0xrawsec/golang-etw layout'u) çapraz denetlendi.
// ÖNEMLİ: yalnız Sizeof değil, ProcessTrace'in okuduğu kritik alan ofsetlerini
// de kontrol et — aksi halde eksik/fazla bir alan (bkz. LogInstanceGuid union)
// toplam boyutu tesadüfen tutturursa fark edilmez.
func checkLayout() error {
	type want struct {
		name string
		got  uintptr
		exp  uintptr
	}
	for _, w := range []want{
		{"eventTraceProperties", unsafe.Sizeof(eventTraceProperties{}), 120},
		{"enableTraceParameters", unsafe.Sizeof(enableTraceParameters{}), 48},
		{"timeZoneInformation", unsafe.Sizeof(timeZoneInformation{}), 172},
		{"traceLogfileHeader", unsafe.Sizeof(traceLogfileHeader{}), 280},
		{"traceLogfileHeader.LoggerName off", unsafe.Offsetof(traceLogfileHeader{}.LoggerName), 56},
		{"eventTrace", unsafe.Sizeof(eventTrace{}), 88},
		{"eventTraceLogfileW", unsafe.Sizeof(eventTraceLogfileW{}), 448},
		{"eventHeader", unsafe.Sizeof(eventHeader{}), 80},
		{"eventRecord", unsafe.Sizeof(eventRecord{}), 112},
		{"eventRecord.UserData off", unsafe.Offsetof(eventRecord{}.UserData), 96},
		{"logfile.LogfileHeader off", unsafe.Offsetof(eventTraceLogfileW{}.LogfileHeader), 120},
		{"logfile.EventRecordCallback off", unsafe.Offsetof(eventTraceLogfileW{}.EventRecordCallback), 424},
		{"logfile.Context off", unsafe.Offsetof(eventTraceLogfileW{}.Context), 440},
	} {
		if w.got != w.exp {
			return fmt.Errorf("ETW struct düzeni: %s = %d, beklenen %d", w.name, w.got, w.exp)
		}
	}
	return nil
}

// ─── oturum ───

type etwSession struct {
	name          string
	sessionHandle uint64
	traceHandle   uint64
	propsBuf      []byte  // canlı tutulmalı
	cbPtr         uintptr // NewCallback sonucu — canlı tutulmalı
	lost          atomic.Uint64
	stopped       atomic.Bool
}

// etwProvider, oturuma etkinleştirilecek bir sağlayıcı.
type etwProvider struct {
	GUID     windows.GUID
	Keywords uint64 // MatchAnyKeyword
}

// etwEvent, ProcessTrace callback'inden tüketiciye geçen olay (UserData kopyası
// callback dışında geçerlidir).
type etwEvent struct {
	Provider windows.GUID
	ID       uint16
	PID      uint32
	Data     []byte
}

func startETWSession(name string, providers ...etwProvider) (*etwSession, error) {
	if err := checkLayout(); err != nil {
		return nil, err
	}
	s := &etwSession{name: name}
	if err := s.start(providers); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *etwSession) newProps() (*eventTraceProperties, []byte) {
	name16 := utf16z(s.name)
	sz := int(unsafe.Sizeof(eventTraceProperties{})) + len(name16)*2 + 2
	buf := make([]byte, sz)
	p := (*eventTraceProperties)(unsafe.Pointer(&buf[0]))
	p.Wnode.BufferSize = uint32(sz)
	p.Wnode.ClientContext = 1 // QPC
	p.Wnode.Flags = wnodeFlagTracedGUID
	p.LogFileMode = eventTraceRealTimeMode
	p.LoggerNameOffset = uint32(unsafe.Sizeof(eventTraceProperties{}))
	p.BufferSize = 128 // KB
	p.MinimumBuffers = 8
	p.MaximumBuffers = 64
	p.FlushTimer = 1
	return p, buf
}

func (s *etwSession) start(providers []etwProvider) error {
	name16, err := windows.UTF16PtrFromString(s.name)
	if err != nil {
		return err
	}

	p, buf := s.newProps()
	s.propsBuf = buf
	r, _, _ := procStartTraceW.Call(
		uintptr(unsafe.Pointer(&s.sessionHandle)),
		uintptr(unsafe.Pointer(name16)),
		uintptr(unsafe.Pointer(p)),
	)
	if r == errAlreadyExists {
		s.controlStop() // eski oturumu kapat
		p, buf = s.newProps()
		s.propsBuf = buf
		r, _, _ = procStartTraceW.Call(
			uintptr(unsafe.Pointer(&s.sessionHandle)),
			uintptr(unsafe.Pointer(name16)),
			uintptr(unsafe.Pointer(p)),
		)
	}
	if r != 0 {
		return fmt.Errorf("StartTraceW: %w", windows.Errno(r))
	}

	for i := range providers {
		guid := providers[i].GUID
		params := enableTraceParameters{Version: 2}
		r, _, _ = procEnableTrace2.Call(
			uintptr(s.sessionHandle),
			uintptr(unsafe.Pointer(&guid)),
			eventControlEnableProvider,
			traceLevelVerbose,
			uintptr(providers[i].Keywords),
			0, // MatchAllKeyword
			0, // Timeout
			uintptr(unsafe.Pointer(&params)),
		)
		if r != 0 {
			s.controlStop()
			return fmt.Errorf("EnableTraceEx2(%v): %w", guid, windows.Errno(r))
		}
	}
	return nil
}

// Process, ProcessTrace ile olay pompasını çalıştırır — Stop() çağrılana kadar
// bloke eder. wanted(id) true dönen her olay için cb, olayın kopyasıyla
// çağrılır (kopya callback dışında geçerlidir).
func (s *etwSession) Process(wanted func(id uint16) bool, cb func(etwEvent)) {
	name16, _ := windows.UTF16PtrFromString(s.name)

	logfile := &eventTraceLogfileW{
		LoggerName:       name16,
		ProcessTraceMode: procTraceModeRealTime | procTraceModeEventRecord,
	}
	goCB := func(rec *eventRecord) uintptr {
		if guidEqual(&rec.EventHeader.ProviderId, &rtLostEventGUID) {
			s.lost.Add(1)
			return 0
		}
		id := rec.EventHeader.EventDescriptor.Id
		if !wanted(id) {
			return 0
		}
		var blob []byte
		if n := int(rec.UserDataLength); rec.UserData != nil && n > 0 {
			blob = make([]byte, n)
			copy(blob, unsafe.Slice(rec.UserData, n))
		}
		cb(etwEvent{
			Provider: rec.EventHeader.ProviderId,
			ID:       id,
			PID:      rec.EventHeader.ProcessId,
			Data:     blob,
		})
		return 0
	}
	s.cbPtr = windows.NewCallback(goCB)
	logfile.EventRecordCallback = s.cbPtr

	th, _, _ := procOpenTraceW.Call(uintptr(unsafe.Pointer(logfile)))
	if th == invalidTraceHandle {
		slog.Error("ETW OpenTraceW başarısız", "err", windows.GetLastError())
		return
	}
	s.traceHandle = uint64(th)

	// ProcessTrace, Stop()'ta CloseTrace çağrılana dek bloke eder. Anında
	// dönüş = consumer hiç olay pompalamadı (genelde struct ABI uyuşmazlığı).
	r, _, _ := procProcessTrace.Call(uintptr(unsafe.Pointer(&s.traceHandle)), 1, 0, 0)
	if r != 0 && r != errCancelled {
		slog.Warn("ETW ProcessTrace sonlandı", "err", windows.Errno(r))
	}
}

func (s *etwSession) Stop() {
	if s.stopped.Swap(true) {
		return
	}
	if s.traceHandle != 0 {
		_, _, _ = procCloseTrace.Call(uintptr(s.traceHandle)) // → ProcessTrace döner
	}
	s.controlStop()
}

func (s *etwSession) controlStop() {
	name16, err := windows.UTF16PtrFromString(s.name)
	if err != nil {
		return
	}
	p, _ := s.newProps()
	_, _, _ = procControlTraceW.Call(
		uintptr(s.sessionHandle),
		uintptr(unsafe.Pointer(name16)),
		uintptr(unsafe.Pointer(p)),
		eventTraceControlStop,
	)
}

func (s *etwSession) LostEvents() uint64 { return s.lost.Load() }

func guidEqual(a, b *windows.GUID) bool {
	return a.Data1 == b.Data1 && a.Data2 == b.Data2 && a.Data3 == b.Data3 && a.Data4 == b.Data4
}

func utf16z(s string) []uint16 {
	u, _ := windows.UTF16FromString(s)
	return u
}
