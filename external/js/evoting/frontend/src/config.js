module.exports = {
  tequila: {
    hostname: 'tequila.epfl.ch'
  },
  ldap: {
    hostname: 'ldap.epfl.ch'
  },
  // masterKey: new Uint8Array([241, 5, 168, 215, 97, 244, 172, 216, 228, 227, 216, 29, 128, 155, 237, 218, 81, 121, 237, 115, 173, 247, 108, 129, 141, 228, 53, 205, 13, 127, 91, 237])
  // masterKey: new Uint8Array([16, 250, 251, 75, 119, 95, 216, 63, 50, 174, 8, 89, 40, 22, 169, 3, 88, 71, 186, 78, 123, 38, 232, 18, 83, 119, 98, 104, 209, 72, 168, 211]),
  masterKey: new Uint8Array([84, 185, 18, 159, 67, 167, 151, 153, 235, 4, 231, 254, 81, 207, 164, 41, 214, 224, 204, 141, 237, 223, 155, 156, 93, 236, 68, 101, 97, 244, 138, 89]),
  breadcrumbs: [
    { text: 'EPFL', href: 'https://epfl.ch' },
    { text: 'EPFL Assembly', href: 'https://ae.epfl.ch' },
    { text: 'Elections 2018', href: 'https://ae.epfl.ch/elections' }
  ]
}
