"use strict";

const chai = require("chai");
const expect = chai.expect;

const curve = require("../../lib/curve");

describe("curves", () => {
  it("are all listed", () => {
    const allCurves = curve.availableCurves();
    expect(allCurves).to.include("edwards25519");
    expect(allCurves).to.include("p256");
  });

  it("can be created by name", () => {
    const ed25519 = curve.newCurve("edwards25519");
    expect(ed25519).to.be.an.instanceof(curve.edwards25519.Curve);
    expect(ed25519.point().pick()).to.be.an.instanceof(
      curve.edwards25519.Point
    );
    expect(() => curve.newCurve("unknown")).to.throw("not known");
  });
});
