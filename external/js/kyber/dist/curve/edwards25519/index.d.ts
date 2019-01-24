import Ed25519 from "./curve";
import Ed25519Point from "./point";
import Ed25519Scalar from "./scalar";
declare const _default: {
    Point: typeof Ed25519Point;
    Scalar: typeof Ed25519Scalar;
    Curve: typeof Ed25519;
};
export default _default;
