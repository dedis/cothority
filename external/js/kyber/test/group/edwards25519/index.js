const EdDSA = require("elliptic").eddsa;
const ec = new EdDSA("ed25519");
const kyber = require("../../../index.js");
const edwards25519 = kyber.curve.edwards25519;
const BN = require("bn.js");
const util = require("../../util");
const unhexlify = util.unhexlify;
const hexToUint8Array = util.hexToUint8Array;
const PRNG = util.PRNG;
const assert = require("chai").assert;
const fs = require("fs");
const crypto = require("crypto");

describe("edwards25519", () => {
  const curve = new edwards25519.Curve();
  const prng = new PRNG(42);
  const setSeed = prng.setSeed.bind(prng);
  const randomBytes = prng.pseudoRandomBytes.bind(prng);

  it("should return the name of the curve", () => {
    assert(curve.string() === "Ed25519", "Curve name is not correct");
  });

  it("scalarLen should return the length of scalar", () => {
    assert(curve.scalarLen() === 32, "Scalar length not correct");
  });

  it("pointLen should return the length of point", () => {
    assert(curve.pointLen() === 32, "Point length not correct");
  });

  it("scalar should return a scalar", () => {
    assert(
      curve.scalar().constructor === edwards25519.Scalar,
      "Scalar not returned"
    );
  });

  it("point should return a point", () => {
    assert(
      curve.point().constructor === edwards25519.Point,
      "Point not returned"
    );
  });

  describe("point", () => {
    // prettier-ignore
    const bytes = new Uint8Array([5, 69, 248, 173, 171, 254, 19, 253, 143, 140, 146, 174, 26, 128, 3, 52, 106, 55, 112, 245, 62, 127, 42, 93, 0, 81, 47, 177, 30, 25, 39, 198]);
    describe("marshalSize", () => {
      it("should return the marshal data length", () => {
        assert.strictEqual(curve.point().marshalSize(), 32);
      });
    });
    describe("string", () => {
      it("should print the string representation of a point", () => {
        let point = curve.point();
        point.unmarshalBinary(bytes);

        let target =
          "0545f8adabfe13fd8f8c92ae1a8003346a3770f53e7f2a5d00512fb11e1927c6";
        assert.strictEqual(point.string(), target);
      });

      it("should print the string representation of a null point", () => {
        let point = curve.point().null();
        let target =
          "0100000000000000000000000000000000000000000000000000000000000000";

        assert.strictEqual(point.string(), target);
      });
    });
    describe("unmarshalBinary", () => {
      // in curve25519 package
      it("should retrieve the correct point", () => {
        let point = curve.point();
        point.unmarshalBinary(bytes);

        const targetX = new BN(
          "20279109212561463415328781515723337704810403260034987929787219898780302754141",
          10
        );
        const targetY = new BN(
          "4627191eb12f51005d2a7f3ef570376a3403801aae928c8ffd13feabadf84505",
          16
        );
        assert.equal(
          point.ref.point.getX().cmp(targetX),
          0,
          "X Coordinate unequal"
        );
        assert.equal(
          point.ref.point.getY().cmp(targetY),
          0,
          "Y Coordinate unequal"
        );
      });

      it("should work with a zero buffer", () => {
        const bytes = new Uint8Array(curve.pointLen());
        let point = curve.point();
        point.unmarshalBinary(bytes);
        let target = new BN(
          "2b8324804fc1df0b2b4d00993dfbd7a72f431806ad2fe478c4ee1b274a0ea0b0",
          16
        );
        assert.strictEqual(point.ref.point.getX().cmp(target), 0);
      });

      it("should throw an exception on an invalid point", () => {
        let b = randomBytes(curve.pointLen());
        let point = curve.point();
        assert.throws(() => {
          point.unmarshalBinary(b);
        }, Error);
      });

      it("should throw an exception if input is not Uint8Array", () => {
        assert.throws(
          () => curve.point().unmarshalBinary([1, 2, 3]),
          TypeError
        );
      });
    });

    describe("marshalBinary", () => {
      it("should marshal the point according to spec", () => {
        let point = curve.point();
        point.unmarshalBinary(bytes);

        assert.deepEqual(point.marshalBinary(), bytes);
      });
    });

    describe("equal", () => {
      it("should return true for equal points and false otherwise", () => {
        let x = new BN(
          "39139857753964535406422970543512609321558395110412588924902544519776250623903",
          10
        );
        let y = new BN(
          "22969022198784600445029639705880320580068058102470723316833310869386179114437",
          10
        );
        let a = new edwards25519.Point(curve,x, y);
        let b = new edwards25519.Point(curve,x, y);
        assert.isTrue(a.equal(b), "equals returns false for two equal points");
        assert.isFalse(
          a.equal(new edwards25519.Point(curve,)),
          "equal returns true for two unequal points"
        );
      });
    });

    describe("null", () => {
      it("should set the point to the null element", () => {
        let point = curve.point().null();
        assert.isTrue(point.ref.point.isInfinity(), "isInfinity returns false");
      });
    });

    describe("base", () => {
      it("should set the point to the base point", () => {
        let point = curve.point().base();
        let gx = new BN(
          "15112221349535400772501151409588531511454012693041857206046113283949847762202",
          10
        );
        let gy = new BN(
          "46316835694926478169428394003475163141307993866256225615783033603165251855960",
          10
        );
        assert.strictEqual(
          point.ref.point.x.fromRed().cmp(gx),
          0,
          "x coord != gx"
        );
        assert.strictEqual(
          point.ref.point.y.fromRed().cmp(gy),
          0,
          "y coord != gy"
        );
      });
    });

    describe("pick", () => {
      beforeEach(() => {
        setSeed(42);
      });

      it("should pick a random point on the curve", () => {
        let point = curve.point().pick();
        assert.isTrue(ec.curve.validate(point.ref.point), "point not on curve");
      });

      it("should pick a random point with a callback", () => {
        let point = curve.point().pick(randomBytes);
        let x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        let y = new BN(
          "a56e25b707524601a885ebfba304e858a3fbf457a7a204b90fb9c2a888fbe467",
          16,
          "le"
        );
        let target = new edwards25519.Point(curve,x, y);

        assert.isTrue(point.equal(target), "point != target");
      });
    });

    describe("set", () => {
      it("should point the receiver to another Point object", () => {
        let x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        let y = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );
        let a = new edwards25519.Point(curve,x, y);
        let b = curve.point().set(a);

        assert.isTrue(a.equal(b), "a != b");
        a.base();
        assert.isTrue(a.equal(b), "a != b");
      });
    });

    describe("clone", () => {
      it("should clone the point object", () => {
        let x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        let y = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );

        let a = new edwards25519.Point(curve,x, y);
        let b = a.clone();

        assert.isTrue(a.equal(b), "a != b");
        a.base();
        assert.isFalse(a.equal(b), "a == b");
      });
    });

    describe("embedLen", () => {
      it("should return the embed length of point", () => {
        assert.strictEqual(curve.point().embedLen(), 29, "Wrong embed length");
      });
    });

    describe("embed", () => {
      beforeEach(() => {
        setSeed(42);
      });
      it("should throw a TypeError if data is not a Uint8Array", () => {
        assert.throws(() => {
          curve.point().embed(123);
        }, TypeError);
      });

      it("should throw an Error if data length > embedLen", () => {
        let point = curve.point();
        let data = new Uint8Array(point.embedLen() + 1);
        assert.throws(() => {
          point.embed(data);
        }, Error);
      });

      it("should embed data with length < embedLen", () => {
        let data = new Uint8Array([1, 2, 3, 4, 5, 6]);
        let point = curve.point().embed(data, randomBytes);

        let x = new BN(
          "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
          16
        );
        let y = new BN(
          "060102030405061c52e43010209ff7fed474ea3cf04f3fd69648912bc28c7819",
          16,
          "le"
        );
        let target = new edwards25519.Point(curve,x, y);
        assert.isTrue(point.equal(target), "point != target");
      });

      it("should embed data with length = embedLen", () => {
        // prettier-ignore
        let data = new Uint8Array([68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73]);
        let point = curve.point().embed(data, randomBytes);

        let x = new BN(
          "7950d89da1c23ee557dd6e8fa59f12393366f2e9bdb4e69f880b44c272426bbf",
          16
        );
        let y = new BN(
          "441c49444544534944454453494445445349444544534944454453494445441d",
          16
        );

        let target = new edwards25519.Point(curve,x, y);
        assert.isTrue(point.equal(target), "point != target");
      });
    });

    describe("data", () => {
      it("should extract embedded data", () => {
        let x = new BN(
          "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
          16
        );
        let y = new BN(
          "19788cc22b914896d63f4ff03cea74d4fef79f201030e4521c06050403020106",
          16
        );
        let point = new edwards25519.Point(curve,x, y);
        let data = new Uint8Array([1, 2, 3, 4, 5, 6]);
        assert.deepEqual(point.data(), data, "data returned wrong values");
      });

      it("should throw an Error on embeded length > embedLen", () => {
        let point = curve.point().base();
        assert.throws(() => {
          point.data();
        }, Error);
      });
    });

    describe("add", () => {
      it("should add two points", () => {
        setSeed(42);

        let x3 = new BN(
          "57011823e7bf4d7bd56128c11a52c5410f50539ff235bc67599234eebae6689e",
          16
        );
        let y3 = new BN(
          "4d68a958f33a76d991fa87a9e6adc7187708bc6ad159e2fb23236669e5994e4a",
          16
        );

        let p1 = curve.point().pick(randomBytes);
        let p2 = curve.point().pick(randomBytes);
        let p3 = new edwards25519.Point(curve,x3, y3);
        let sum = curve.point().add(p1, p2);
        // a + b = b + a
        let sum2 = curve.point().add(p2, p1);

        assert.isTrue(ec.curve.validate(sum.ref.point), "sum not on curve");
        assert.isTrue(sum.equal(p3), "sum != p3");
        assert.isTrue(ec.curve.validate(sum2.ref.point), "sum2 not on curve");
        assert.isTrue(sum2.equal(p3), "sum2 != p3");
      });
    });

    describe("sub", () => {
      it("should subtract two points", () => {
        setSeed(42);

        let x3 = new BN(
          "680270df9cdcfdd959589470e041eecf05135db52ddd926135e250ae32fa68f9",
          16
        );
        let y3 = new BN(
          "49eef2fec1766d2e0d291d757e9e9882a6ae8ed74fa70e609e02aa09c446eb6e",
          16
        );

        let p1 = curve.point().pick(randomBytes);
        let p2 = curve.point().pick(randomBytes);
        let p3 = new edwards25519.Point(curve,x3, y3);
        let diff = curve.point().sub(p1, p2);

        assert.isTrue(
          ec.curve.validate(diff.ref.point),
          "diff not on curve"
        );
        assert.isTrue(diff.equal(p3), "diff != p3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        setSeed(42);
        let x2 = new BN(
          "49a7874e659489fcd9d66c460e0fa1df50369f5652226573765b3a67ce2ab410",
          16
        );
        let y2 = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );

        let p1 = curve.point().pick(randomBytes);
        let p2 = new edwards25519.Point(curve,x2, y2);
        let neg = curve.point().neg(p1);

        assert.isTrue(ec.curve.validate(neg.ref.point), "neg not on curve");
        assert.isTrue(neg.equal(p2), "neg != p2");
      });

      it("should negate null point", () => {
        let nullPoint = curve.point().null();
        let negNull = curve.point().neg(nullPoint);

        assert.isTrue(negNull.equal(nullPoint), "negNull != nullPoint");
      });
    });

    describe("mul", () => {
      it("should multiply p by scalar s", () => {
        setSeed(42);
        let x2 = new BN(
          "f570b26f180eeeb764e5a39b381b93ac44e2b7c3e93692440fe1a392ccdc5300",
          16,
          "le"
        );
        let y2 = new BN(
          "153780eef951eb9b02530195136fd27609e7d7c7c6b3e0820bdb121f44eec518",
          16,
          "le"
        );

        let p1 = curve.point().pick(randomBytes);
        let buf = new Uint8Array([5, 10]);
        let s = curve.scalar().setBytes(buf);
        let prod = curve.point().mul(s, p1);
        let p2 = new edwards25519.Point(curve,x2, y2);

        assert.isTrue(
          ec.curve.validate(prod.ref.point),
          "prod not on curve"
        );
        assert.isTrue(prod.equal(p2), "prod != p2");
      });

      it("should multiply with base point if no point is passed", () => {
        let base = curve.point().base();
        let three = new Uint8Array([3]);
        let threeScalar = curve.scalar().setBytes(three);
        let target = curve.point().mul(threeScalar, base);
        let threeBase = curve.point().mul(threeScalar);

        assert.isTrue(
          ec.curve.validate(threeBase.ref.point),
          "threeBase not on curve"
        );
        assert.isTrue(threeBase.equal(target), "target != threeBase");
      });
    });
  });

  describe("scalar", () => {
    // prettier-ignore
    let b1 = new Uint8Array([101, 216, 110, 23, 127, 7, 203, 250, 206, 170, 55, 91, 97, 239, 222, 159, 41, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 171]);
    // prettier-ignore
    let b2 = new Uint8Array([88, 146, 91, 18, 158, 90, 102, 25, 82, 85, 219, 232, 60, 253, 138, 65, 183, 2, 157, 218, 70, 58, 193, 179, 212, 232, 104, 98, 125, 202, 176, 9]);

    describe("setBytes", () => {
      it("should set the scalar reading bytes from little endian array", () => {
        let bytes = new Uint8Array([2, 4, 8, 10]);
        let s = curve.scalar().setBytes(bytes);
        let target = new BN("0a080402", 16);
        assert.strictEqual(s.ref.arr.fromRed().cmp(target), 0);
      });

      it("should throw TypeError when b is not Uint8Array", () => {
        assert.throws(() => {
          curve.scalar().setBytes(1234);
        }, TypeError);
      });

      it("should reduce to number to mod Q", () => {
        // prettier-ignore
        let bytes = new Uint8Array([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        let s = curve.scalar().setBytes(bytes);
        let target = new BN(
          "0ffffffffffffffffffffffffffffffec6ef5bf4737dcf70d6ec31748d98951c",
          16
        );

        assert.strictEqual(s.ref.arr.fromRed().cmp(target), 0);
      });
    });

    describe("equal", () => {
      it("should return true for equal scalars and false otherwise", () => {
        let bytes = new Uint8Array([241, 51, 4]);
        let s1 = curve.scalar().setBytes(bytes);
        let s2 = curve.scalar().setBytes(bytes);

        assert.isTrue(s1.equal(s2), "s1 != s2");
        assert.isFalse(s1.equal(curve.scalar(), "s1 == 0"));
      });
    });

    describe("zero", () => {
      it("should set the scalar to 0", () => {
        let s = curve.scalar().zero();
        let target = new BN(0, 16);
        assert.strictEqual(s.ref.arr.fromRed().cmp(target), 0);
      });
    });

    describe("set", () => {
      it("should make the receiver point to a scalar", () => {
        let bytes = new Uint8Array([1, 2, 4, 52]);
        let s1 = curve.scalar().setBytes(bytes);
        let s2 = curve.scalar().set(s1);
        let zero = curve.scalar().zero();

        assert.isTrue(s1.equal(s2), "s1 != s2");
        s1.zero();
        assert.isTrue(s1.equal(s2), "s1 != s2");
        assert.isTrue(s1.equal(zero), "s1 != 0");
      });
    });

    describe("clone", () => {
      it("should clone a scalar", () => {
        let bytes = new Uint8Array([1, 2, 4, 52]);
        let s1 = curve.scalar().setBytes(bytes);
        let s2 = s1.clone();

        assert.isTrue(s1.equal(s2), "s1 != s2");
        s1.zero();
        assert.isFalse(s1.equal(s2), "s1 == s2");
      });
    });

    describe("setInt64", () => {});

    describe("add", () => {
      it("should add two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let sum = curve.scalar().add(s1, s2);

        // prettier-ignore
        let target = new Uint8Array([142, 79, 58, 43, 251, 31, 103, 75, 235, 66, 111, 67, 13, 48, 213, 251, 223, 252, 30, 150, 83, 181, 96, 87, 34, 5, 98, 17, 87, 61, 173, 5]);
        let s3 = curve.scalar();
        s3.unmarshalBinary(target);

        assert.isTrue(sum.equal(s3), "sum != s3");
      });
    });

    describe("sub", () => {
      it("should subtract two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let diff = curve.scalar().sub(s1, s2);
        // prettier-ignore
        let target = new Uint8Array([203, 254, 120, 99, 217, 205, 172, 112, 29, 53, 176, 20, 114, 47, 158, 141, 113, 247, 228, 224, 197, 64, 222, 239, 120, 51, 144, 76, 92, 168, 75, 2]);
        let s3 = curve.scalar();
        s3.unmarshalBinary(target);
        assert.isTrue(diff.equal(s3), "diff != s3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        let s1 = curve.scalar().setBytes(b1);
        let neg = curve.scalar().neg(s1);

        // prettier-ignore
        let target = new Uint8Array([202, 66, 33, 231, 162, 58, 255, 205, 102, 18, 108, 165, 47, 205, 181, 69, 215, 5, 126, 68, 243, 132, 96, 92, 178, 227, 6, 81, 38, 141, 3, 4]);
        let s2 = curve.scalar();
        s2.unmarshalBinary(target);

        assert.isTrue(neg.equal(s2), "neg != s2");
      });
    });

    describe("bytes", () => {
      // to be removed in #277
      xit("should return the bytes in big-endian representation", () => {
        let s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        let bytes = s1.bytes();
        // prettier-ignore
        let target = new Uint8Array([171, 252, 114, 217, 174, 249, 28, 77, 163, 159, 123, 12, 187, 129, 250, 41, 159, 222, 239, 97, 91, 55, 170, 206, 250, 203, 7, 127, 23, 110, 216, 101]);

        assert.deepEqual(bytes, target);
      });
    });

    describe("one", () => {
      let one = curve.scalar().one();

      it("should set the scalar to one", () => {
        let bytes = one.marshalBinary()
        let target = new Uint8Array([1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
        assert.deepEqual(target, bytes);
      });
    });

    describe("mul", () => {
      it("should multiply two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let prod = curve.scalar().mul(s1, s2);

        // prettier-ignore
        let target = new Uint8Array([34, 211, 107, 76, 100, 86, 13, 215, 123, 147, 172, 207, 230, 235, 139, 24, 48, 176, 64, 192, 65, 15, 67, 221, 226, 55, 42, 236, 84, 151, 8, 7]);
        let s3 = curve.scalar();
        s3.unmarshalBinary(target);

        assert.isTrue(prod.equal(s3), "mul != s3");
      });
    });

    describe("div", () => {
      it("should divide two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let quotient = curve.scalar().div(s1, s2);

        // prettier-ignore
        let target = new Uint8Array([145, 191, 30, 22, 157, 168, 12, 162, 220, 120, 243, 189, 108, 219, 155, 180, 153, 9, 224, 106, 128, 43, 50, 228, 38, 190, 218, 139, 185, 250, 4, 4]);
        let s3 = curve.scalar();
        s3.unmarshalBinary(target);

        assert.isTrue(quotient.equal(s3), "quotient != s3");
      });
    });
    describe("inv", () => {
      it("should compute the inverse modulo n of scalar", () => {
        let s1 = curve.scalar().setBytes(b1);
        let inv = curve.scalar().inv(s1);

        // prettier-ignore
        let target = new Uint8Array([154, 16, 208, 201, 223, 62, 219, 72, 103, 81, 202, 115, 69, 207, 192, 15, 46, 182, 202, 37, 102, 233, 116, 118, 239, 127, 234, 84, 12, 32, 206, 5]);
        let s2 = curve.scalar();
        s2.unmarshalBinary(target);

        assert.isTrue(inv.equal(s2), "inv != s2");
      });
    });

    describe("pick", () => {
      it("should pick a random scalar", () => {
        setSeed(42);
        let s1 = curve.scalar().pick(randomBytes);

        // prettier-ignore
        let bytes = new Uint8Array([231, 30, 187, 110, 193, 139, 10, 170, 126, 79, 112, 41, 212, 167, 34, 46, 227, 253, 241, 189, 81, 181, 199, 179, 13, 151, 183, 143, 196, 244, 208, 1]);
        let target = curve.scalar();
        target.unmarshalBinary(bytes);

        assert.isTrue(s1.equal(target));
      });
    });

    describe("marshalBinary", () => {
      it("should return the marshalled representation of scalar", () => {
        let s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        let m = s1.marshalBinary();
        // prettier-ignore
        let target = new Uint8Array([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);

        assert.deepEqual(m, target);
      });
    });

    describe("unmarshalBinary", () => {
      it("should convert marshalled representation to scalar", () => {
        let s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        let target = new Uint8Array([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);
        let bytes = s1.marshalBinary();

        assert.deepEqual(bytes, target);
      });

      it("should throw an error if input is not Uint8Array", () => {
        let s1 = curve.scalar();
        assert.throws(() => {
          s1.unmarshalBinary(123);
        }, TypeError);
      });

      // not in case of Edwards25519
      xit("should throw an error if input > q", () => {
        let s1 = curve.scalar();
        assert.throws(() => {
          s1.unmarshalBinary(b1);
        }, Error);
      });

      it("should throw an error if input size > marshalSize", () => {
        let s1 = curve.scalar();
        let data = new Uint8Array(s1.marshalSize() + 1);
        assert.throws(() => {
          s1.unmarshalBinary(data);
        }, Error);
      });
    });

    describe("string", () => {
      it("should print the string representation of a scalar", () => {
        let s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        let target = "2391d4757728138a6f8a8bfdae2c29cf28fa81bb0c7b9fa34d1cf9aed972fc0b";
        assert.strictEqual(s1.string(), target);
      });

      // TODO: discrepency
      xit("should print the string representation of zero scalar", () => {
        let s1 = curve.scalar().zero();
        let target = "";
        assert.strictEqual(s1.string(), target);
      });

      it("should print the string representation of one scalar", () => {
        let s1 = curve.scalar().one();
        let target = "0100000000000000000000000000000000000000000000000000000000000000";
        assert.strictEqual(s1.string(), target);
      });
    });
  });

  /**
   * Test vectors from http://ed25519.cr.yp.to/python/sign.input
   */
  describe("ed25519 test vectors", () => {
    let lines;
    before((done) => {
      fs.readFile(__dirname + "/sign.input", "utf-8", (err, data) => {
        lines = data.split("\n");
        done();
      });
    });

    function testFactory(i) {
      it("vector " + i, () => {
        let parts = lines[i].split(":");
        let hash = crypto.createHash("sha512");
        let digest = hash.update(unhexlify(parts[0].substring(0, 64))).digest()
        digest = digest.slice(0, 32);
        digest[0] &= 0xf8;
        digest[31] &= 0x3f;
        digest[31] |= 0x40;
        let sk = new BN(digest.slice(0, 32), 16, "le");
        // using hexToUint8Array until
        // https://github.com/indutny/bn.js/issues/175 is resolved
        let pk = new BN(hexToUint8Array(parts[1]), 16, "le");
        let s = curve.scalar();
        s.unmarshalBinary(Uint8Array.from(sk.toArray("le")));
        let p = curve.point();
        p.unmarshalBinary(Uint8Array.from(pk.toArray("le")));

        let target = curve.point().mul(s);

        assert.isTrue(p.equal(target));
      });
    }

    for(let i = 0; i < 1024; i++) {
      testFactory(i);
    }
  });
});
