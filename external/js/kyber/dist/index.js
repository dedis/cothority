"use strict";
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (Object.hasOwnProperty.call(mod, k)) result[k] = mod[k];
    result["default"] = mod;
    return result;
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const curve = __importStar(require("./curve"));
exports.curve = curve;
const sign = __importStar(require("./sign"));
exports.sign = sign;
const point_factory_1 = __importDefault(require("./point-factory"));
exports.PointFactory = point_factory_1.default;
exports.default = {
    curve,
    sign,
    PointFactory: point_factory_1.default,
};
