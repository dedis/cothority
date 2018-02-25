export const scipersToUint8Array = scipers => {
  return Uint8Array.from([].concat(...scipers.map(sciper => {
    const ret = []
    let tmp = parseInt(sciper)
    for (let i = 0; i < 3; i++) {
      ret.push(tmp & 0xff)
      tmp = tmp >> 8
    }
    return ret
  })))
}

/**
 * This assumes that the encoded data was indeed a byte array initially
 */
export const b64ToUint8Array = data => {
  const b64String = data.replace('/-/g', '/')
  return Uint8Array.from(atob(b64String).split(',').map(x => parseInt(x)))
}
