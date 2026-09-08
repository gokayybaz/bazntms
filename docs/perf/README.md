# Performans / ölçek koşuları (Faz 21)

Bu dizin, kapasite doğrulama koşularının çıktılarını tutar.

## `runs/`

`scripts/loadtest.sh <profil>` her koşuda `runs/<utc>-<profil>.md` üretir:
loadgen istemci-tarafı özeti + hub `/metrics` deltaları + profil eşiklerine
karşı PASS/FAIL tablosu.

`runs/` **git tarafından yok sayılır** — çoğu koşu yereldir ve gürültülüdür.
Kapasite raporuna (`docs/CAPACITY.md`, S21.16) kanıt olacak koşuların
raporları `git add -f` ile bilinçli eklenir.

## `pprof/`

`scripts/profile.sh` (S21.6) yük altında toplanan CPU/heap/goroutine/block/mutex
profillerini `pprof/<utc>/` altına yazar. Bu da yok sayılır; raporlanan
darboğazların profilleri bilinçli eklenir.

## Profiller

`loadtest/profiles/` — `baseline` (hızlı regresyon), `target` (enterprise
kapasite hedefleri), `burst` (200k flow/sn patlama). Eşikleri
`scripts/perf_summary.py` uygular.
