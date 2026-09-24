/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useAuthStore } from '@/stores/auth-store'
import { Footer } from '@/components/layout/components/footer'
import { Capabilities } from './sections/capabilities'
import { GetStarted } from './sections/get-started'
import { AboutHero } from './sections/hero'
import { OpenSource } from './sections/open-source'

/**
 * Default about page rendered when the administrator has not customized the
 * About option. Mirrors the default home page's landing structure.
 */
export function AboutLandingPage() {
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user

  return (
    <div className='about-page'>
      <AboutHero isAuthenticated={isAuthenticated} />
      <Capabilities />
      <OpenSource />
      <GetStarted isAuthenticated={isAuthenticated} />
      <Footer />
    </div>
  )
}
