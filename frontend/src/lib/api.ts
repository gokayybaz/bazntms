// errText, bir başarısız Response'tan okunabilir bir hata mesajı çıkarır.
// Sunucu bazı uçlarda düz metin (http.Error), bazılarında JSON {error:…} döner.
export async function errText(res: Response): Promise<string> {
  const body = await res.text()
  try {
    const j = JSON.parse(body)
    return j.error || body
  } catch {
    return body.trim() || `HTTP ${res.status}`
  }
}
