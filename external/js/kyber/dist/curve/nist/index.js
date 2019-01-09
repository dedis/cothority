"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const point_1 = __importDefault(require("./point"));
const scalar_1 = __importDefault(require("./scalar"));
const curve_1 = __importDefault(require("./curve"));
const params_1 = __importDefault(require("./params"));
exports.default = {
    Point: point_1.default,
    Scalar: scalar_1.default,
    Curve: curve_1.default,
    Params: params_1.default,
};
