"use strict";
var __rest = (this && this.__rest) || function (s, e) {
    var t = {};
    for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p) && e.indexOf(p) < 0)
        t[p] = s[p];
    if (s != null && typeof Object.getOwnPropertySymbols === "function")
        for (var i = 0, p = Object.getOwnPropertySymbols(s); i < p.length; i++) if (e.indexOf(p[i]) < 0)
            t[p[i]] = s[p[i]];
    return t;
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const elliptic_1 = __importDefault(require("elliptic"));
const bn_js_1 = __importDefault(require("bn.js"));
const point_1 = __importDefault(require("./point"));
const scalar_1 = __importDefault(require("./scalar"));
class Weierstrass {
    constructor(config) {
        let { name, bitSize, gx, gy } = config, options = __rest(config, ["name", "bitSize", "gx", "gy"]);
        this.name = name;
        options["g"] = [new bn_js_1.default(gx, 16, "le"), new bn_js_1.default(gy, 16, "le")];
        for (let k in options) {
            if (k === "g") {
                continue;
            }
            options[k] = new bn_js_1.default(options[k], 16, "le");
        }
        this.curve = new elliptic_1.default.curve.short(options);
        this.bitSize = bitSize;
        this.redN = bn_js_1.default.red(options.n);
    }
    coordLen() {
        return (this.bitSize + 7) >> 3;
    }
    /**
    * Returns the size in bytes of a scalar
    */
    scalarLen() {
        return (this.curve.n.bitLength() + 7) >> 3;
    }
    /**
    * Returns the size in bytes of a point
    */
    scalar() {
        return new scalar_1.default(this, this.redN);
    }
    /**
    * Returns the size in bytes of a point
    */
    pointLen() {
        // ANSI X9.62: 1 header byte plus 2 coords
        return this.coordLen() * 2 + 1;
    }
    /**
    * Returns a new Point
    */
    point() {
        return new point_1.default(this);
    }
    /**
     * Get the name of the curve
     */
    string() {
        return this.name;
    }
}
exports.default = Weierstrass;
