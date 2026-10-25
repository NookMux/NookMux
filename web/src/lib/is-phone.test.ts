import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { isPhoneUA } from './is-phone'

const IPHONE_SAFARI =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1'
const ANDROID_CHROME =
  'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36'
const WECHAT_ANDROID =
  'Mozilla/5.0 (Linux; Android 13; 2210132C) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/116.0.0.0 Mobile Safari/537.36 XWEB/1160065 MMWEBSDK/20231202'
const ANDROID_TABLET =
  'Mozilla/5.0 (Linux; Android 13; SM-X910) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36'
const IPAD_LEGACY =
  'Mozilla/5.0 (iPad; CPU OS 12_1_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.0 Mobile/15E148 Safari/604.1'
const IPAD_DESKTOP_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15'
const PHONE_DESKTOP_MODE =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36'
const WINDOWS_CHROME =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36'
const MAC_SAFARI =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15'

describe('isPhoneUA', () => {
  test('matches iPhone and Android phone UAs', () => {
    assert.equal(isPhoneUA(IPHONE_SAFARI), true)
    assert.equal(isPhoneUA(ANDROID_CHROME), true)
    assert.equal(isPhoneUA(WECHAT_ANDROID), true)
  })

  test('lets desktop-mode phone UAs through', () => {
    assert.equal(isPhoneUA(PHONE_DESKTOP_MODE), false)
  })

  test('lets tablets through', () => {
    assert.equal(isPhoneUA(ANDROID_TABLET), false)
    assert.equal(isPhoneUA(IPAD_LEGACY), false)
    assert.equal(isPhoneUA(IPAD_DESKTOP_UA), false)
  })

  test('lets desktop browsers through', () => {
    assert.equal(isPhoneUA(WINDOWS_CHROME), false)
    assert.equal(isPhoneUA(MAC_SAFARI), false)
  })
})
