import BN = require("bn.js");
import NistPoint from "./point";
import NistScalar from "./scalar";
import Weierstrass from "./curve";
declare const _default: {
    Point: typeof NistPoint;
    Scalar: typeof NistScalar;
    Curve: typeof Weierstrass;
    Params: {
        p256: {
            name: string;
            bitSize: number;
            p: BN.default;
            a: BN.default;
            b: BN.default;
            n: BN.default;
            gx: BN.default;
            gy: BN.default;
        };
    };
};
export default _default;
