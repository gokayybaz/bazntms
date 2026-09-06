// Card — geriye dönük alias. TUI dönüşümünde tek konteyner Panel oldu;
// mevcut `import { Card } from './Card'` çağrıları kırılmasın diye burada
// yeniden ihraç ediliyor. Çağrı yerleri dokunuldukça doğrudan Panel'e geçiyor;
// bu dosya S17.31'de silinecek.
export { Panel as Card } from './Panel'
