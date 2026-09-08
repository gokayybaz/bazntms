# bazNTMS Agent MSI — Npcap sessiz kurulum yardimcisi.
#
# NEDEN: Windows'ta L7 (TLS SNI / HTTP Host) uygulama gorunurlugu pcap
# arka ucunu, o da Npcap'i gerektirir (surec trafigi + DNS ETW ile Npcap'siz
# de calisir, ama L7 calismaz). Agent v1.3.0'dan beri Windows'ta method: pcap
# ile gelir; bu betik kurulum sirasinda Npcap'i (yoksa) indirip sessizce kurar.
#
# NASIL CAGRILIR: bazntms-agent.wxs icindeki `RunNpcapSetup` deferred
# CustomAction'i (SYSTEM, Impersonate=no, Return=ignore). Betik ASLA hata
# donmez (her yoldan `exit 0`) — Npcap kurulamazsa MSI yine de basariyla biter,
# agent ETW'ye duser ve log'a net bir uyari yazar.
#
# GUNCELLEME: yeni Npcap surumu ciktiginda $NpcapVersion + $NpcapSha256'yi
# guncelleyin (deploy/config/... ve docs/RELEASE-RUNBOOK.md'de not var).
# SHA-256 disinda ayrica Authenticode imzasi (Nmap Software LLC / Insecure.Com
# LLC) dogrulanir — bu, surum atlansa bile saglam kalan asil butunluk kontrolu.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$NpcapVersion = '1.88'
$NpcapUrl     = "https://npcap.com/dist/npcap-$NpcapVersion.exe"
$NpcapSha256  = 'A2F4EC1E5EA353FF67EFD24B2EBF081BA44532410FAE8D5E146AF0310AA4F56B'
$SignerOk     = @('Nmap Software LLC', 'Insecure.Com LLC')

$LogDir = Join-Path $env:ProgramData 'bazntms'
$Log    = Join-Path $LogDir 'npcap-install.log'

function Write-Log($msg) {
    $line = "{0}  {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg
    try {
        if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir -Force | Out-Null }
        Add-Content -Path $Log -Value $line -Encoding UTF8
    } catch { }
    Write-Output $line
}

function Test-NpcapInstalled {
    if (Get-Service -Name 'npcap' -ErrorAction SilentlyContinue) { return $true }
    $sysNpcap = Join-Path $env:SystemRoot 'System32\Npcap\wpcap.dll'
    if (Test-Path $sysNpcap) { return $true }
    if (Test-Path 'HKLM:\SOFTWARE\WOW6432Node\Npcap') { return $true }
    if (Test-Path 'HKLM:\SOFTWARE\Npcap') { return $true }
    return $false
}

try {
    Write-Log "Npcap kurulum yardimcisi basladi (hedef surum $NpcapVersion)."

    if (Test-NpcapInstalled) {
        Write-Log 'Npcap zaten kurulu — atlaniyor.'
        exit 0
    }

    # TLS 1.2 (eski Windows PowerShell varsayilanlari SSL3/TLS1.0 deneyebilir)
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch { }

    $tmp = Join-Path $env:TEMP ("npcap-{0}.exe" -f $NpcapVersion)
    if (Test-Path $tmp) { Remove-Item $tmp -Force -ErrorAction SilentlyContinue }

    $downloaded = $false
    for ($i = 1; $i -le 3 -and -not $downloaded; $i++) {
        try {
            Write-Log "Indiriliyor ($i/3): $NpcapUrl"
            Invoke-WebRequest -Uri $NpcapUrl -OutFile $tmp -UseBasicParsing -TimeoutSec 120
            $downloaded = $true
        } catch {
            Write-Log "Indirme denemesi $i basarisiz: $($_.Exception.Message)"
            Start-Sleep -Seconds 5
        }
    }
    if (-not $downloaded) {
        Write-Log 'Npcap indirilemedi — agent ETW ile calisacak (L7 yok). Elle: https://npcap.com'
        exit 0
    }

    $hash = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash
    if ($hash -ne $NpcapSha256) {
        Write-Log "SHA-256 uyusmuyor (beklenen $NpcapSha256, gelen $hash) — kurulum iptal, dosya siliniyor."
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        exit 0
    }
    Write-Log "SHA-256 dogrulandi: $hash"

    $sig = Get-AuthenticodeSignature -FilePath $tmp
    $subject = ''
    if ($sig -and $sig.SignerCertificate) { $subject = $sig.SignerCertificate.Subject }
    $signerMatch = $false
    foreach ($s in $SignerOk) { if ($subject -like "*$s*") { $signerMatch = $true } }
    if ($sig.Status -ne 'Valid' -or -not $signerMatch) {
        Write-Log "Authenticode dogrulanamadi (status=$($sig.Status), signer=$subject) — kurulum iptal."
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        exit 0
    }
    Write-Log "Authenticode dogrulandi: $subject"

    Write-Log 'Npcap sessiz kuruluyor: /S'
    $p = Start-Process -FilePath $tmp -ArgumentList '/S' -Wait -PassThru
    Write-Log "Npcap kurulum cikis kodu: $($p.ExitCode)"
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue

    if (Test-NpcapInstalled) {
        Write-Log 'Npcap kurulumu dogrulandi — L7 gorunurlugu etkin.'
    } else {
        Write-Log 'Npcap kurulumdan sonra tespit edilemedi — agent ETW ile calisacak (L7 yok).'
    }
    exit 0
} catch {
    Write-Log "Beklenmeyen hata: $($_.Exception.Message)"
    exit 0
}
