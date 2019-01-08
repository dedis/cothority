const { eddsa } = require("elliptic");
const ec = new eddsa("ed25519");
const BN = require("bn.js");
const fs = require("fs");
const crypto = require("crypto");
const kyber = require("../../../dist/index.js");
const { unhexlify, hexToUint8Array, PRNG } = require("../../util");

const {
  curve: { edwards25519 }
} = kyber;

describe("edwards25519", () => {
  const curve = new edwards25519.Curve();
  const prng = new PRNG(42);

  it("should return the name of the curve", () => {
    expect(curve.string()).toBe("Ed25519", "Curve name is not correct");
  });

  it("scalarLen should return the length of scalar", () => {
    expect(curve.scalarLen()).toBe(32, "Scalar length not correct");
  });

  it("pointLen should return the length of point", () => {
    expect(curve.pointLen()).toBe(32, "Point length not correct");
  });

  it("scalar should return a scalar", () => {
    expect(curve.scalar()).toEqual(jasmine.any(edwards25519.Scalar));
  });

  it("point should return a point", () => {
    expect(curve.point()).toEqual(jasmine.any(edwards25519.Point));
  });

  describe("point", () => {
    // prettier-ignore
    const bytes = Buffer.from([5, 69, 248, 173, 171, 254, 19, 253, 143, 140, 146, 174, 26, 128, 3, 52, 106, 55, 112, 245, 62, 127, 42, 93, 0, 81, 47, 177, 30, 25, 39, 198]);

    it("should return the marshal data length", () => {
      expect(curve.point().marshalSize()).toBe(32);
    });

    it("should print the string representation of a point", () => {
      const point = curve.point();
      point.unmarshalBinary(bytes);

      expect(point.string()).toBe(
        "0545f8adabfe13fd8f8c92ae1a8003346a3770f53e7f2a5d00512fb11e1927c6"
      );
    });

    it("should print the string representation of a null point", () => {
      const point = curve.point().null();

      expect(point.string()).toBe(
        "0100000000000000000000000000000000000000000000000000000000000000"
      );
    });

    it("should retrieve the correct point", () => {
      const point = curve.point();
      point.unmarshalBinary(bytes);

      const targetX = new BN(
        "20279109212561463415328781515723337704810403260034987929787219898780302754141",
        10
      );
      const targetY = new BN(
        "4627191eb12f51005d2a7f3ef570376a3403801aae928c8ffd13feabadf84505",
        16
      );

      expect(point.ref.point.getX().cmp(targetX)).toBe(
        0,
        "X Coordinate unequal"
      );
      expect(point.ref.point.getY().cmp(targetY)).toBe(
        0,
        "Y Coordinate unequal"
      );
    });

    it("should work with a zero buffer", () => {
      const bytes = Buffer.alloc(curve.pointLen(), 0);
      const point = curve.point();
      point.unmarshalBinary(bytes);
      const target = new BN(
        "2b8324804fc1df0b2b4d00993dfbd7a72f431806ad2fe478c4ee1b274a0ea0b0",
        16
      );

      expect(point.ref.point.getX().cmp(target)).toBe(0);
    });

    it("should throw an exception on an invalid point", () => {
      prng.setSeed(42);
      const b = prng.pseudoRandomBytes(curve.pointLen());
      const point = curve.point();

      console.log(b);
      expect(() => point.unmarshalBinary(b)).toThrow();
    });

    it("should throw an exception if input is not Uint8Array", () => {
      expect(() => curve.point().unmarshalBinary([1, 2, 3])).toThrow(
        new TypeError("argument must be of type buffer")
      );
    });

    it("should marshal the point according to spec", () => {
      let point = curve.point();
      point.unmarshalBinary(bytes);

      expect(point.marshalBinary()).toEqual(bytes);
    });

    it("should return true for equal points and false otherwise", () => {
      const x = new BN(
        "39139857753964535406422970543512609321558395110412588924902544519776250623903",
        10
      );
      const y = new BN(
        "22969022198784600445029639705880320580068058102470723316833310869386179114437",
        10
      );
      const a = new edwards25519.Point(curve, x, y);
      const b = new edwards25519.Point(curve, x, y);

      expect(a.equal(b)).toBeTruthy(
        "equals returns false for two equal points"
      );
      expect(a.equal(new edwards25519.Point(curve))).toBeFalsy(
        "equal returns true for two unequal points"
      );
    });

    it("should set the point to the null element", () => {
      const point = curve.point().null();

      expect(point.ref.point.isInfinity()).toBeTruthy(
        "isInfinity returns false"
      );
    });

    it("should set the point to the base point", () => {
      const point = curve.point().base();
      const gx = new BN(
        "15112221349535400772501151409588531511454012693041857206046113283949847762202",
        10
      );
      const gy = new BN(
        "46316835694926478169428394003475163141307993866256225615783033603165251855960",
        10
      );

      expect(point.ref.point.x.fromRed().cmp(gx)).toBe(0, "x coord != gx");
      expect(point.ref.point.y.fromRed().cmp(gy)).toBe(0, "y coord != gy");
    });

    describe("pick", () => {
      beforeEach(() => prng.setSeed(42));

      it("should pick a random point on the curve", () => {
        const point = curve.point().pick();

        expect(ec.curve.validate(point.ref.point)).toBeTruthy(
          "point not on curve"
        );
      });

      it("should pick a random point with a callback", () => {
        const point = curve.point().pick(prng.pseudoRandomBytes);
        const x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        const y = new BN(
          "a56e25b707524601a885ebfba304e858a3fbf457a7a204b90fb9c2a888fbe467",
          16,
          "le"
        );
        const target = new edwards25519.Point(curve, x, y);

        expect(point.equal(target)).toBeTruthy("point != target");
      });
    });

    describe("set", () => {
      it("should point the receiver to another Point object", () => {
        const x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        const y = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );
        const a = new edwards25519.Point(curve, x, y);
        const b = curve.point().set(a);

        expect(a.equal(b)).toBeTruthy("a != b");
        a.base();
        expect(a.equal(b)).toBeTruthy("a != b");
      });
    });

    describe("clone", () => {
      it("should clone the point object", () => {
        const x = new BN(
          "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
          16
        );
        const y = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );

        const a = new edwards25519.Point(curve, x, y);
        const b = a.clone();

        expect(a.equal(b)).toBeTruthy();
        a.base();
        expect(a.equal(b)).toBeFalsy();
      });
    });

    describe("embedLen", () => {
      it("should return the embed length of point", () => {
        expect(curve.point().embedLen()).toBe(29, "Wrong embed length");
      });
    });

    describe("embed", () => {
      beforeEach(() => prng.setSeed(42));
      it("should throw a TypeError if data is not a Uint8Array", () => {
        expect(() => curve.point().embed(123)).toThrow();
      });

      it("should throw an Error if data length > embedLen", () => {
        const point = curve.point();
        const data = new Uint8Array(point.embedLen() + 1);

        expect(() => point.embed(data)).toThrow();
      });

      it("should embed data with length < embedLen", () => {
        const data = Buffer.from([1, 2, 3, 4, 5, 6]);
        const point = curve.point().embed(data, prng.pseudoRandomBytes);

        const x = new BN(
          "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
          16
        );
        const y = new BN(
          "060102030405061c52e43010209ff7fed474ea3cf04f3fd69648912bc28c7819",
          16,
          "le"
        );
        const target = new edwards25519.Point(curve, x, y);

        expect(point.equal(target)).toBeTruthy("point != target");
      });

      it("should embed data with length = embedLen", () => {
        // prettier-ignore
        const data = Buffer.from([68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73]);
        const point = curve.point().embed(data, prng.pseudoRandomBytes);

        const x = new BN(
          "7950d89da1c23ee557dd6e8fa59f12393366f2e9bdb4e69f880b44c272426bbf",
          16
        );
        const y = new BN(
          "441c49444544534944454453494445445349444544534944454453494445441d",
          16
        );

        const target = new edwards25519.Point(curve, x, y);

        expect(point.equal(target)).toBeTruthy("point != target");
      });
    });

    describe("data", () => {
      it("should extract embedded data", () => {
        const x = new BN(
          "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
          16
        );
        const y = new BN(
          "19788cc22b914896d63f4ff03cea74d4fef79f201030e4521c06050403020106",
          16
        );
        const point = new edwards25519.Point(curve, x, y);
        const data = Buffer.from([1, 2, 3, 4, 5, 6]);

        expect(point.data()).toEqual(data, "data returned wrong values");
      });

      it("should throw an Error on embeded length > embedLen", () => {
        const point = curve.point().base();

        expect(() => point.data()).toThrow();
      });
    });

    describe("add", () => {
      it("should add two points", () => {
        prng.setSeed(42);

        const x3 = new BN(
          "57011823e7bf4d7bd56128c11a52c5410f50539ff235bc67599234eebae6689e",
          16
        );
        const y3 = new BN(
          "4d68a958f33a76d991fa87a9e6adc7187708bc6ad159e2fb23236669e5994e4a",
          16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = curve.point().pick(prng.pseudoRandomBytes);
        const p3 = new edwards25519.Point(curve, x3, y3);
        const sum = curve.point().add(p1, p2);
        // a + b = b + a
        const sum2 = curve.point().add(p2, p1);

        expect(ec.curve.validate(sum.ref.point)).toBeTruthy("sum not on curve");
        expect(sum.equal(p3)).toBeTruthy("sum != p3");
        expect(ec.curve.validate(sum2.ref.point)).toBeTruthy(
          "sum2 not on curve"
        );
        expect(sum2.equal(p3)).toBeTruthy("sum2 != p3");
      });
    });

    describe("sub", () => {
      it("should subtract two points", () => {
        prng.setSeed(42);

        const x3 = new BN(
          "680270df9cdcfdd959589470e041eecf05135db52ddd926135e250ae32fa68f9",
          16
        );
        const y3 = new BN(
          "49eef2fec1766d2e0d291d757e9e9882a6ae8ed74fa70e609e02aa09c446eb6e",
          16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = curve.point().pick(prng.pseudoRandomBytes);
        const p3 = new edwards25519.Point(curve, x3, y3);
        const diff = curve.point().sub(p1, p2);

        expect(ec.curve.validate(diff.ref.point)).toBeTruthy(
          "diff not on curve"
        );
        expect(diff.equal(p3)).toBeTruthy("diff != p3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        prng.setSeed(42);
        const x2 = new BN(
          "49a7874e659489fcd9d66c460e0fa1df50369f5652226573765b3a67ce2ab410",
          16
        );
        const y2 = new BN(
          "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
          16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = new edwards25519.Point(curve, x2, y2);
        const neg = curve.point().neg(p1);

        expect(ec.curve.validate(neg.ref.point)).toBeTruthy("neg not on curve");
        expect(neg.equal(p2)).toBeTruthy("neg != p2");
      });

      it("should negate null point", () => {
        const nullPoint = curve.point().null();
        const negNull = curve.point().neg(nullPoint);

        expect(negNull.equal(nullPoint)).toBeTruthy("negNull != nullPoint");
      });
    });

    describe("mul", () => {
      it("should multiply p by scalar s", () => {
        prng.setSeed(42);
        const x2 = new BN(
          "f570b26f180eeeb764e5a39b381b93ac44e2b7c3e93692440fe1a392ccdc5300",
          16,
          "le"
        );
        const y2 = new BN(
          "153780eef951eb9b02530195136fd27609e7d7c7c6b3e0820bdb121f44eec518",
          16,
          "le"
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const buf = Buffer.from([5, 10]);
        const s = curve.scalar().setBytes(buf);
        const prod = curve.point().mul(s, p1);
        const p2 = new edwards25519.Point(curve, x2, y2);

        expect(ec.curve.validate(prod.ref.point)).toBeTruthy(
          "prod not on curve"
        );
        expect(prod.equal(p2)).toBeTruthy("prod != p2");
      });

      it("should multiply with base point if no point is passed", () => {
        const base = curve.point().base();
        const three = Buffer.from([3]);
        const threeScalar = curve.scalar().setBytes(three);
        const target = curve.point().mul(threeScalar, base);
        const threeBase = curve.point().mul(threeScalar);

        expect(ec.curve.validate(threeBase.ref.point)).toBeTruthy(
          "threeBase not on curve"
        );
        expect(threeBase.equal(target)).toBeTruthy("target != threeBase");
      });
    });
  });

  describe("scalar", () => {
    // prettier-ignore
    const b1 = Buffer.from([101, 216, 110, 23, 127, 7, 203, 250, 206, 170, 55, 91, 97, 239, 222, 159, 41, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 171]);
    // prettier-ignore
    const b2 = Buffer.from([88, 146, 91, 18, 158, 90, 102, 25, 82, 85, 219, 232, 60, 253, 138, 65, 183, 2, 157, 218, 70, 58, 193, 179, 212, 232, 104, 98, 125, 202, 176, 9]);

    describe("setBytes", () => {
      it("should set the scalar reading bytes from little endian array", () => {
        const bytes = Buffer.from([2, 4, 8, 10]);
        const s = curve.scalar().setBytes(bytes);
        const target = new BN("0a080402", 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
      });

      it("should throw TypeError when b is not a buffer", () => {
        expect(() => curve.scalar().setBytes(1234)).toThrow();
      });

      it("should reduce to number to mod Q", () => {
        // prettier-ignore
        const bytes = Buffer.from([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        const s = curve.scalar().setBytes(bytes);
        const target = new BN(
          "0ffffffffffffffffffffffffffffffec6ef5bf4737dcf70d6ec31748d98951c",
          16
        );

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
      });
    });

    describe("equal", () => {
      it("should return true for equal scalars and false otherwise", () => {
        const bytes = Buffer.from([241, 51, 4]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().setBytes(bytes);

        expect(s1.equal(s2)).toBeTruthy("s1 != s2");
        expect(s1.equal(curve.scalar())).toBeFalsy("s1 == 0");
      });
    });

    describe("zero", () => {
      it("should set the scalar to 0", () => {
        const s = curve.scalar().zero();
        const target = new BN(0, 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
      });
    });

    describe("set", () => {
      it("should make the receiver point to a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().set(s1);
        const zero = curve.scalar().zero();

        expect(s1.equal(s2)).toBeTruthy("s1 != s2");
        s1.zero();
        expect(s1.equal(s2)).toBeTruthy("s1 != s2");
        expect(s1.equal(zero)).toBeTruthy("s1 != 0");
      });
    });

    describe("clone", () => {
      it("should clone a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = s1.clone();

        expect(s1.equal(s2)).toBeTruthy("s1 != s2");
        s1.zero();
        expect(s1.equal(s2)).toBeFalsy("s1 == s2");
      });
    });

    describe("add", () => {
      it("should add two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const sum = curve.scalar().add(s1, s2);

        // prettier-ignore
        const target = Buffer.from([142, 79, 58, 43, 251, 31, 103, 75, 235, 66, 111, 67, 13, 48, 213, 251, 223, 252, 30, 150, 83, 181, 96, 87, 34, 5, 98, 17, 87, 61, 173, 5]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(sum.equal(s3)).toBeTruthy("sum != s3");
      });
    });

    describe("sub", () => {
      it("should subtract two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const diff = curve.scalar().sub(s1, s2);
        // prettier-ignore
        const target = Buffer.from([203, 254, 120, 99, 217, 205, 172, 112, 29, 53, 176, 20, 114, 47, 158, 141, 113, 247, 228, 224, 197, 64, 222, 239, 120, 51, 144, 76, 92, 168, 75, 2]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(diff.equal(s3)).toBeTruthy("diff != s3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        const s1 = curve.scalar().setBytes(b1);
        const neg = curve.scalar().neg(s1);

        // prettier-ignore
        const target = Buffer.from([202, 66, 33, 231, 162, 58, 255, 205, 102, 18, 108, 165, 47, 205, 181, 69, 215, 5, 126, 68, 243, 132, 96, 92, 178, 227, 6, 81, 38, 141, 3, 4]);
        const s2 = curve.scalar();
        s2.unmarshalBinary(target);

        expect(neg.equal(s2)).toBeTruthy("neg != s2");
      });
    });

    describe("bytes", () => {
      // to be removed in #277
      xit("should return the bytes in big-endian representation", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        const bytes = s1.bytes();
        // prettier-ignore
        const target = Buffer.from([171, 252, 114, 217, 174, 249, 28, 77, 163, 159, 123, 12, 187, 129, 250, 41, 159, 222, 239, 97, 91, 55, 170, 206, 250, 203, 7, 127, 23, 110, 216, 101]);

        expect(bytes).toEqual(target);
      });
    });

    describe("one", () => {
      const one = curve.scalar().one();

      it("should set the scalar to one", () => {
        const bytes = one.marshalBinary();
        // prettier-ignore
        const target = Buffer.from([1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);

        expect(bytes).toEqual(target);
      });
    });

    describe("mul", () => {
      it("should multiply two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const prod = curve.scalar().mul(s1, s2);

        // prettier-ignore
        const target = Buffer.from([34, 211, 107, 76, 100, 86, 13, 215, 123, 147, 172, 207, 230, 235, 139, 24, 48, 176, 64, 192, 65, 15, 67, 221, 226, 55, 42, 236, 84, 151, 8, 7]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(prod.equal(s3)).toBeTruthy("mul != s3");
      });
    });

    describe("div", () => {
      it("should divide two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const quotient = curve.scalar().div(s1, s2);

        // prettier-ignore
        const target = Buffer.from([145, 191, 30, 22, 157, 168, 12, 162, 220, 120, 243, 189, 108, 219, 155, 180, 153, 9, 224, 106, 128, 43, 50, 228, 38, 190, 218, 139, 185, 250, 4, 4]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(quotient.equal(s3)).toBeTruthy("quotient != s3");
      });
    });

    describe("inv", () => {
      it("should compute the inverse modulo n of scalar", () => {
        const s1 = curve.scalar().setBytes(b1);
        const inv = curve.scalar().inv(s1);

        // prettier-ignore
        const target = Buffer.from([154, 16, 208, 201, 223, 62, 219, 72, 103, 81, 202, 115, 69, 207, 192, 15, 46, 182, 202, 37, 102, 233, 116, 118, 239, 127, 234, 84, 12, 32, 206, 5]);
        const s2 = curve.scalar();
        s2.unmarshalBinary(target);

        expect(inv.equal(s2)).toBeTruthy();
      });
    });

    describe("pick", () => {
      it("should pick a random scalar", () => {
        prng.setSeed(42);
        const s1 = curve.scalar().pick(prng.pseudoRandomBytes);

        // prettier-ignore
        const bytes = Buffer.from([231, 30, 187, 110, 193, 139, 10, 170, 126, 79, 112, 41, 212, 167, 34, 46, 227, 253, 241, 189, 81, 181, 199, 179, 13, 151, 183, 143, 196, 244, 208, 1]);
        const target = curve.scalar();
        target.unmarshalBinary(bytes);

        expect(s1.equal(target)).toBeTruthy();
      });
    });

    describe("marshalBinary", () => {
      it("should return the marshalled representation of scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        const m = s1.marshalBinary();
        // prettier-ignore
        const target = Buffer.from([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);

        expect(m).toEqual(target);
      });
    });

    describe("unmarshalBinary", () => {
      it("should convert marshalled representation to scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        const target = Buffer.from([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);
        const bytes = s1.marshalBinary();

        expect(bytes).toEqual(target);
      });

      it("should throw an error if input is not a buffer", () => {
        const s1 = curve.scalar();

        expect(() => s1.unmarshalBinary(123)).toThrow();
      });

      // not in case of Edwards25519
      xit("should throw an error if input > q", () => {
        const s1 = curve.scalar();

        expect(() => s1.unmarshalBinary(b1)).toThrow();
      });

      it("should throw an error if input size > marshalSize", () => {
        const s1 = curve.scalar();
        const data = Buffer.allocUnsafe(s1.marshalSize() + 1);

        expect(() => s1.unmarshalBinary(data)).toThrow();
      });
    });

    describe("string", () => {
      it("should print the string representation of a scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        const target = "2391d4757728138a6f8a8bfdae2c29cf28fa81bb0c7b9fa34d1cf9aed972fc0b";

        expect(s1.toString()).toBe(target);
      });

      // TODO: discrepency
      xit("should print the string representation of zero scalar", () => {
        let s1 = curve.scalar().zero();
        let target = "";
        assert.strictEqual(s1.string(), target);
      });

      it("should print the string representation of one scalar", () => {
        const s1 = curve.scalar().one();
        const target =
          "0100000000000000000000000000000000000000000000000000000000000000";

        expect(s1.toString()).toBe(target);
      });
    });
  });

  /**
   * Test vectors from http://ed25519.cr.yp.to/python/sign.input
   */
  describe("ed25519 test vectors", () => {
    let lines;
    beforeAll(done => {
      fs.readFile(__dirname + "/sign.input", "utf-8", (err, data) => {
        lines = data.split("\n");
        done();
      });
    });

    function testFactory(i) {
      it("vector " + i, () => {
        let parts = lines[i].split(":");
        let hash = crypto.createHash("sha512");
        let digest = hash.update(unhexlify(parts[0].substring(0, 64))).digest();
        digest = digest.slice(0, 32);
        digest[0] &= 0xf8;
        digest[31] &= 0x3f;
        digest[31] |= 0x40;
        let sk = new BN(digest.slice(0, 32), 16, "le");
        // using hexToUint8Array until
        // https://github.com/indutny/bn.js/issues/175 is resolved
        let pk = new BN(hexToUint8Array(parts[1]), 16, "le");
        let s = curve.scalar();
        s.unmarshalBinary(Buffer.from(sk.toArray("le")));
        let p = curve.point();
        p.unmarshalBinary(Buffer.from(pk.toArray("le")));

        let target = curve.point().mul(s);

        expect(p.equal(target)).toBeTruthy();
      });
    }

    for (let i = 0; i < 1024; i++) {
      testFactory(i);
    }
  });
});
