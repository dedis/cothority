"use strict";

const chai = require("chai");
const expect = chai.expect;

const cothority = require("../../lib");
const proto = cothority.protobuf;
const kyber = require("@dedis/kyber-js");

const helpers = require("../helpers.js");

const curve = new kyber.curve.edwards25519.Curve();

describe("skipchain client", () => {
  it("correctly verifies a link", () => {
    const n = 5;
    const kps = helpers.keypairs(curve, n);
    const myKeyPair = {
      priv: curve.scalar().pick(),
      pub: curve.point().pick()
    };
    expect(myKeyPair.pub instanceof kyber.Point).to.be.true;
    expect(myKeyPair.priv instanceof kyber.Scalar).to.be.true;
    const ids = helpers.roster(curve, kps);
  });
});
