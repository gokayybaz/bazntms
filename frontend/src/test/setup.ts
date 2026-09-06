import '@testing-library/jest-dom/vitest'

// jsdom scrollIntoView'ı implemente etmiyor — TuiTable seçili satırı görünür
// tutmak için çağırıyor.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}
