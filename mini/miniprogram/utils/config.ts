export const TEST_API_ORIGIN = 'http://127.0.0.1:8080'
export const PRODUCTION_API_ORIGIN = 'https://omr.superwhys.top'

// Localhost is reachable only from the developer tool on this machine.
const localTest =
  wx.getAccountInfoSync?.()?.miniProgram.envVersion === 'develop' && wx.getSystemInfoSync?.()?.platform === 'devtools'
export const API_ORIGIN = localTest ? TEST_API_ORIGIN : PRODUCTION_API_ORIGIN
