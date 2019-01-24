import Ed25519 from "./curve";
import Ed25519Point from "./point";
import Ed25519Scalar from "./scalar";

export default {
    Point: Ed25519Point,
    Scalar: Ed25519Scalar,
    Curve: Ed25519,
}