package capture

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/gopacket/pcap"
)

// TestNewEngineZeroSnapshot, Start hic cagrilmadan (gercek pcap arayuzu
// gerekmeden) Engine'in guvenli, tutarli bir bos-durum snapshot'i
// dondurdugunu dogrular — Running=false, sifir sayaclar, bos slice'lar
// (nil degil, JSON serilestirmede "null" yerine "[]" gorunmesi icin).
func TestNewEngineZeroSnapshot(t *testing.T) {
	e := NewEngine()
	s := e.Snapshot()

	if s.Running {
		t.Error("Start cagrilmadan Running=true olmamali")
	}
	if s.TotalPackets != 0 || s.TotalBytes != 0 || s.Dropped != 0 {
		t.Errorf("sifir sayaclar beklenirdi: %+v", s)
	}
	if s.Protocols == nil {
		t.Error("Protocols nil olmamali (bos map bekleniyor)")
	}
}

// TestEngineIsLocal, refreshLocalNets'in makinenin gercek arayuzlerinden
// topladigi aglara gore loopback IP'yi (127.0.0.1 her zaman bir arayuzun
// parcasidir) yerel olarak tanidigini, rastgele bir genel IP'yi tanimadigini
// dogrular.
func TestEngineIsLocal(t *testing.T) {
	e := NewEngine()
	if !e.isLocal(net.ParseIP("127.0.0.1")) {
		t.Error("127.0.0.1 yerel olarak taninmali")
	}
	if e.isLocal(net.ParseIP("8.8.8.8")) {
		t.Error("8.8.8.8 gibi genel bir IP yerel olarak taninmamali")
	}
}

// TestEngineRollBucket, history diliminin buyudugunu ve historySize sinirini
// astiginda en eski kayitlarin (FIFO) budandigini dogrular.
func TestEngineRollBucket(t *testing.T) {
	e := NewEngine()
	for i := 0; i < historySize+10; i++ {
		e.rollBucket()
	}
	s := e.Snapshot()
	if len(s.History) != historySize {
		t.Fatalf("history %d ile sinirlanmali, gelen: %d", historySize, len(s.History))
	}
}

func TestPortName(t *testing.T) {
	// gopacket'in TCPPort/UDPPort.String()'i "<port>(<isim>)" bicimini
	// kullanir (orn. "80(http)") — portName bunu oldugu gibi dondurur.
	cases := []struct {
		port uint16
		want string
	}{
		{80, "80(http)"},
		{443, "443(https)"},
		{22, "22(ssh)"},
	}
	for _, c := range cases {
		if got := portName(c.port); got != c.want {
			t.Errorf("portName(%d) = %q, beklenen %q", c.port, got, c.want)
		}
	}
	// gopacket'in bilmedigi/kayitli olmayan yuksek bir port: bos donmeli
	// (numeric-gorunumlu isim, isNumericName tarafindan elenir)
	if got := portName(54321); got != "" {
		t.Errorf("bilinmeyen port icin bos beklenirdi, gelen: %q", got)
	}
}

func TestIsNumericName(t *testing.T) {
	if !isNumericName("", 80) {
		t.Error("bos string numeric sayilmali")
	}
	if !isNumericName("80", 80) {
		t.Error("portun kendisiyle ayni string numeric sayilmali")
	}
	if isNumericName("http", 80) {
		t.Error("gercek bir servis adi numeric sayilmamali")
	}
}

func TestIsTimeoutErr(t *testing.T) {
	if !isTimeoutErr(pcap.NextErrorTimeoutExpired) {
		t.Error("pcap.NextErrorTimeoutExpired timeout olarak taninmali")
	}
	if !isTimeoutErr(errors.New("read timed out")) {
		t.Error("\"timed out\" iceren mesaj timeout olarak taninmali")
	}
	if isTimeoutErr(errors.New("izin reddedildi")) {
		t.Error("ilgisiz bir hata timeout sayilmamali")
	}
}

func TestAtomicFlag(t *testing.T) {
	var f atomicFlag
	if f.load() {
		t.Error("varsayilan deger false olmali")
	}
	f.store(true)
	if !f.load() {
		t.Error("store(true) sonrasi load true donmeli")
	}
}

func TestAtomicStr(t *testing.T) {
	var s atomicStr
	if s.load() != "" {
		t.Error("varsayilan deger bos string olmali")
	}
	s.store("eth0")
	if s.load() != "eth0" {
		t.Errorf("store(\"eth0\") sonrasi load \"eth0\" donmeli, gelen: %q", s.load())
	}
}

// TestResolverLookupUnknown, hic kuyruklanmamis/coz(ulmemis) bir IP icin
// Lookup'in (DNS sorgusu beklemeden, gercek agla ugrasmadan) bos string
// dondurdugunu dogrular.
func TestResolverLookupUnknown(t *testing.T) {
	r := newResolver()
	if got := r.Lookup("203.0.113.1"); got != "" {
		t.Errorf("kuyruklanmamis IP icin bos beklenirdi, gelen: %q", got)
	}
}

// TestResolverQueueDoesNotBlockWhenFull, kuyruk kapasitesi (128) dolduktan
// sonra Queue'nun (worker'in tuketmesini beklemeden) bloklanmadan hemen
// donduugunu dogrular — asiri yuklenmede capture donguyu kilitlememeli.
func TestResolverQueueDoesNotBlockWhenFull(t *testing.T) {
	r := &resolver{cache: map[string]string{}, queue: make(chan string, 4)}
	// worker calistirilmadi: kuyruk kimse tarafindan tuketilmiyor
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.Queue("1.2.3.4")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Queue, kuyruk doluyken bloklanmamali (select/default deseni bekleniyordu)")
	}
}
