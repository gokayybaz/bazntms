// bazNTMS eBPF atıf motoru — süreç bazlı bayt sayımı (Faz 20, S20.4).
//
// Yöntem: paket yakalamak yerine çekirdeğin TCP/UDP soket katmanına BTF-tabanlı
// fentry ile bağlanır; her send/recv ÇAĞRISINDA (paket başına değil) o anki
// süreç için bayt ekler. Anahtar: (pid, proto, aile, uzak IP, uzak port).
// Userspace 2-3 sn'de bir haritayı okuyup delta üretir (bkz. S20.5).
//
// Yalnızca fentry + CO-RE kullanılır (pt_regs yok) → nesne mimariden bağımsız.
// Bağlantısız UDP (sendto) uzak adresi şimdilik 0.0.0.0'a düşer — msg_name
// ayrıştırması sonraki bir spike'a bırakıldı.

//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_tracing.h>

#define AF_INET 2
#define AF_INET6 10
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17

#define DNS_MAX 512

char LICENSE[] SEC("license") = "GPL";

struct flow_key {
	__u32 pid;
	__u16 dport; // ağ bayt sırası (big-endian)
	__u8 family; // AF_INET | AF_INET6
	__u8 proto;  // IPPROTO_TCP | IPPROTO_UDP
	__u8 daddr[16];
};

struct flow_stat {
	__u64 bytes_out;
	__u64 bytes_in;
	__u8 comm[16];
};

// dns_event, bir DNS mesajının ham wire baytları — userspace parseDNSNames'e
// verilir (S20.6). Yalnızca YANITLAR yakalanır (skb_consume_udp); yanıt soru
// bölümünü taşıdığından her domain görünür.
struct dns_event {
	__u32 pid;
	__u16 len;
	__u8 comm[16];
	__u8 data[DNS_MAX];
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 16384);
	__type(key, struct flow_key);
	__type(value, struct flow_stat);
} flows SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1 << 18); // 256 KiB
} dns_ring SEC(".maps");

// ringbuf'ta __type yok — dns_event'in BTF'e girmesini zorla (bpf2go -type).
const struct dns_event *_unused_dns_event __attribute__((unused));

static __always_inline void account(struct sock *sk, __u8 proto, __u64 out, __u64 in)
{
	if (!sk)
		return;

	__u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
	if (family != AF_INET && family != AF_INET6)
		return;

	__u32 pid = bpf_get_current_pid_tgid() >> 32;
	if (pid == 0)
		return;

	struct flow_key k = {};
	k.pid = pid;
	k.proto = proto;
	k.family = family;
	k.dport = BPF_CORE_READ(sk, __sk_common.skc_dport);

	if (family == AF_INET) {
		__be32 a = BPF_CORE_READ(sk, __sk_common.skc_daddr);
		if ((a & 0xff) == 127) // 127.0.0.0/8 — pcap ana handle da loopback'i saymaz
			return;
		__builtin_memcpy(k.daddr, &a, sizeof(a));
	} else {
		BPF_CORE_READ_INTO(&k.daddr, sk, __sk_common.skc_v6_daddr.in6_u.u6_addr8);
	}

	struct flow_stat *st = bpf_map_lookup_elem(&flows, &k);
	if (st) {
		if (out)
			__sync_fetch_and_add(&st->bytes_out, out);
		if (in)
			__sync_fetch_and_add(&st->bytes_in, in);
		return;
	}

	struct flow_stat init = {};
	init.bytes_out = out;
	init.bytes_in = in;
	bpf_get_current_comm(&init.comm, sizeof(init.comm));
	bpf_map_update_elem(&flows, &k, &init, BPF_ANY);
}

// capture_dns, bir DNS wire mesajını ringbuf'a kopyalar. payload zaten
// çekirdek belleğinde (skb, UDP başlığı pull edilmiş).
static __always_inline void capture_dns(void *payload, __u32 plen)
{
	if (plen < 12)
		return;
	if (plen > DNS_MAX)
		plen = DNS_MAX;

	struct dns_event *ev = bpf_ringbuf_reserve(&dns_ring, sizeof(*ev), 0);
	if (!ev)
		return;
	ev->pid = bpf_get_current_pid_tgid() >> 32;
	ev->len = (__u16)plen;
	bpf_get_current_comm(&ev->comm, sizeof(ev->comm));
	if (bpf_probe_read_kernel(&ev->data, plen, payload)) {
		bpf_ringbuf_discard(ev, 0);
		return;
	}
	bpf_ringbuf_submit(ev, 0);
}

// dns_socket, bir soketin DNS trafiği taşıyıp taşımadığını söyler: uzak port 53
// VEYA loopback hedef (stub-resolver: systemd-resolved 127.0.0.53, Docker
// gömülü DNS 127.0.0.11 — hedef portu iptables ile değiştirilmiş olabilir).
static __always_inline int dns_socket(struct sock *sk)
{
	__u16 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
	if (dport == bpf_htons(53))
		return 1;
	__u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
	if (family == AF_INET) {
		__be32 daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
		return (daddr & 0xff) == 127; // 127.0.0.0/8 (ağ sırası ilk bayt)
	}
	return 0;
}

SEC("fentry/tcp_sendmsg")
int BPF_PROG(tcp_sendmsg, struct sock *sk, void *msg, __u64 size)
{
	account(sk, IPPROTO_TCP, size, 0);
	return 0;
}

SEC("fentry/tcp_cleanup_rbuf")
int BPF_PROG(tcp_cleanup_rbuf, struct sock *sk, int copied)
{
	if (copied > 0)
		account(sk, IPPROTO_TCP, 0, (__u64)copied);
	return 0;
}

SEC("fentry/udp_sendmsg")
int BPF_PROG(udp_sendmsg, struct sock *sk, void *msg, __u64 len)
{
	account(sk, IPPROTO_UDP, len, 0);
	return 0;
}

SEC("fentry/skb_consume_udp")
int BPF_PROG(skb_consume_udp, struct sock *sk, struct sk_buff *skb, int len)
{
	if (len > 0)
		account(sk, IPPROTO_UDP, 0, (__u64)len);

	// DNS görünürlüğü: skb_consume_udp'de UDP başlığı zaten pull edilmiş →
	// skb->data doğrudan DNS payload'ına bakar, skb->len = payload uzunluğu.
	if (sk && skb && dns_socket(sk)) {
		void *payload = BPF_CORE_READ(skb, data);
		__u32 plen = BPF_CORE_READ(skb, len);
		capture_dns(payload, plen);
	}
	return 0;
}
