"use strict";

const kyber = exports;

kyber.curve = require("./lib/curve");
kyber.sign = require("./lib/sign");
const abstractCls = require("./lib/index.js");

kyber.Group = abstractCls.Group;
kyber.Point = abstractCls.Point;
kyber.Scalar = abstractCls.Scalar;
