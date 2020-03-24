import BN from "bn.js";
import Weierstrass from "./curve";
import Params from "./params";
import NistPoint from "./point";
import NistScalar from "./scalar";

export default {
    Curve: Weierstrass,
    Params,
    Point: NistPoint,
    Scalar: NistScalar,
};
