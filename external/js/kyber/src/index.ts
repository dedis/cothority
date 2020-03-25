import * as curve from "./curve";
import * as pairing from "./pairing";
import PointFactory from "./point-factory";
import * as sign from "./sign";
import { Group, Point, Scalar } from "./suite";

export {
    curve,
    sign,
    pairing,
    PointFactory,
    Group,
    Point,
    Scalar,
};

export default {
    PointFactory,
    curve,
    pairing,
    sign,
};
