import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { AdminGuard } from './AdminGuard'

// S12.1: /yonetim/* admin guard'ı — admin değilse "yetkiniz yok", adminse
// (veya auth kapalıysa) alt rota render olur.
function renderAt(path: string, isAdmin: boolean) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/yonetim" element={<AdminGuard isAdmin={isAdmin} />}>
          <Route path="kullanicilar" element={<div>KULLANICI YÖNETİMİ</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('AdminGuard', () => {
  it('admin değilken "Yetkiniz yok" gösterir, alt rotayı render etmez', () => {
    renderAt('/yonetim/kullanicilar', false)
    expect(screen.getByText('Yetkiniz yok')).toBeInTheDocument()
    expect(screen.queryByText('KULLANICI YÖNETİMİ')).not.toBeInTheDocument()
  })

  it('admin (veya auth kapalı) iken alt rotayı render eder', () => {
    renderAt('/yonetim/kullanicilar', true)
    expect(screen.getByText('KULLANICI YÖNETİMİ')).toBeInTheDocument()
    expect(screen.queryByText('Yetkiniz yok')).not.toBeInTheDocument()
  })
})
