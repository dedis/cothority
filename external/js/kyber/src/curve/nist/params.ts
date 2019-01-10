import BN from "bn.js";

export default {
  p256: {
    name: "P256",
    bitSize: 256,
    p: new BN(
      "ffffffffffffffffffffffff00000000000000000000000001000000ffffffff",
      16,
      "le"
    ),
    // -3 mod p
    a: new BN(
      "fcffffffffffffffffffffff00000000000000000000000001000000ffffffff",
      16,
      "le"
    ),
    b: new BN(
      "4b60d2273e3cce3bf6b053ccb0061d65bc86987655bdebb3e7933aaad835c65a",
      16,
      "le"
    ),
    n: new BN(
      "512563fcc2cab9f3849e17a7adfae6bcffffffffffffffff00000000ffffffff",
      16,
      "le"
    ),
    gx: new BN(
      "96c298d84539a1f4a033eb2d817d0377f240a463e5e6bcf847422ce1f2d1176b",
      16,
      "le"
    ),
    gy: new BN(
      "f551bf376840b6cbce5e316b5733ce2b169e0f7c4aebe78e9b7f1afee242e34f",
      16,
      "le"
    )
  }
};
