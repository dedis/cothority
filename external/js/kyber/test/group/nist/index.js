const kyber = require("../../../index.js");
const nist = kyber.curve.nist;
const BN = require("bn.js");
const assert = require("chai").assert;
const PRNG = require("../../util").PRNG;
const nistVectors = require("./ecdh_test.json");

describe("p256", () => {
  const curve = new nist.Curve(nist.Params.p256);
  const prng = new PRNG(42);
  const setSeed = prng.setSeed.bind(prng);
  const randomBytes = prng.pseudoRandomBytes.bind(prng);

  it("should return the name of the curve", () => {
    assert(curve.string() === "P256", "Curve name is not correct");
  });

  it("scalarLen should return the length of scalar", () => {
    assert(curve.scalarLen() === 32, "Scalar length not correct");
  });

  it("pointLen should return the length of point", () => {
    assert(curve.pointLen() === 65, "Point length not correct");
  });

  it("scalar should return a scalar", () => {
    assert(curve.scalar().constructor === nist.Scalar, "Scalar not returned");
  });

  it("point should return a point", () => {
    assert(curve.point().constructor === nist.Point, "Point not returned");
  });

  describe("point", () => {
    const curve = new nist.Curve(nist.Params.p256);
    // prettier-ignore
    const bytes = new Uint8Array([4, 86, 136, 95, 219, 46, 44, 21, 129, 228, 251, 109, 189, 14, 233, 39, 200, 61, 230, 250, 80, 93, 166, 150, 168, 69, 80, 207, 81, 252, 111, 247, 159, 50, 200, 1, 128, 38, 107, 124, 61, 43, 130, 195, 203, 232, 91, 139, 88, 232, 48, 229, 188, 47, 236, 127, 85, 30, 29, 69, 170, 227, 163, 245, 197]);

    describe("marshalSize", () => {
      it("should return the marshal data length", () => {
        assert.strictEqual(curve.point().marshalSize(), 65);
      });
    });
    describe("string", () => {
      it("should print the string representation of a point", () => {
        let point = curve.point();

        // prettier-ignore
        let target = "(39139857753964535406422970543512609321558395110412588924902544519776250623903,22969022198784600445029639705880320580068058102470723316833310869386179114437)";
        point.unmarshalBinary(bytes);
        assert.strictEqual(point.string(), target);
      });

      it("should print the string representation of a null point", () => {
        let point = curve.point().null();
        let target = "(0,0)";

        assert.strictEqual(point.string(), target);
      });
    });
    describe("unmarshalBinary", () => {
      it("should retrieve the correct point", () => {
        let point = curve.point();
        point.unmarshalBinary(bytes);

        const targetX = new BN(
          "39139857753964535406422970543512609321558395110412588924902544519776250623903",
          10
        );
        const targetY = new BN(
          "22969022198784600445029639705880320580068058102470723316833310869386179114437",
          10
        );
        assert.equal(
          point.ref.point.x.fromRed().cmp(targetX),
          0,
          "X Coordinate unequal"
        );
        assert.equal(
          point.ref.point.y.fromRed().cmp(targetY),
          0,
          "Y Coordinate unequal"
        );
      });

      it("should work with a zero buffer", () => {
        const bytes = new Uint8Array(curve.pointLen());
        bytes[0] = 4;
        let point = curve.point().unmarshalBinary(bytes);
        assert(point.ref.point.isInfinity(), "Point not set to infinity");
      });

      it("should throw an exception on an invalid point", () => {
        let b = Uint8Array.from(bytes);
        b[1] = 11;
        assert.throws(() => curve.point().unmarshalBinary(b), Error);

        b = Uint8Array.from(bytes);
        b[0] = 5;
        assert.throws(() => curve.point().unmarshalBinary(b), Error);
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
        let a = new nist.Point(curve, x, y);
        let b = new nist.Point(curve, x, y);
        assert.isTrue(a.equal(b), "equals returns false for two equal points");
        assert.isFalse(
          a.equal(new nist.Point(curve)),
          "equal returns true for two unequal points"
        );
      });
    });

    describe("null", () => {
      it("should set the point to the null element", () => {
        let point = curve.point().null();
        assert.isNull(point.ref.point.x, "x is not null");
        assert.isNull(point.ref.point.y, "y is not null");
        assert.isTrue(point.ref.point.isInfinity(), "isInfinity returns false");
      });
    });

    describe("base", () => {
      it("should set the point to the base point", () => {
        let point = curve.point().base();
        let gx = new BN(
          "48439561293906451759052585252797914202762949526041747995844080717082404635286",
          10
        );
        let gy = new BN(
          "36134250956749795798585127919587881956611106672985015071877198253568414405109",
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
        assert.isTrue(
          curve.curve.validate(point.ref.point),
          "point not on curve"
        );
      });

      it("should pick a random point with a callback", () => {
        let point = curve.point().pick(randomBytes);
        let x = new BN(
          "20748411802786496192214829937999303328497424855966679862814184095607022057120",
          10
        );
        let y = new BN(
          "25265734854388630912039078646990097162089342591470146115346718827076733979605",
          10
        );
        let target = new nist.Point(curve, x, y);

        assert.isTrue(point.equal(target), "point != target");
      });
    });

    describe("set", () => {
      it("should point the receiver to another Point object", () => {
        let x = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );

        let y = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );
        let a = new nist.Point(curve, x, y);
        let b = curve.point().set(a);

        assert.isTrue(a.equal(b), "a != b");
        a.base();
        assert.isTrue(a.equal(b), "a != b");
      });
    });

    describe("clone", () => {
      it("should clone the point object", () => {
        let x = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );

        let y = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );
        let a = new nist.Point(curve, x, y);
        let b = a.clone();

        assert.isTrue(a.equal(b), "a != b");
        a.base();
        assert.isFalse(a.equal(b), "a == b");
      });
    });

    describe("embedLen", () => {
      it("should return the embed length of point", () => {
        assert.strictEqual(curve.point().embedLen(), 30, "Wrong embed length");
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
          "102139584446102187259397119781836631801548481932383950198227714785215701648902",
          10
        );
        let y = new BN(
          "5861343223324868218838501643187874518624053854803548931377838939237611808222",
          10
        );
        let target = new nist.Point(curve, x, y);
        assert.isTrue(point.equal(target), "point != target");
      });

      it("should embed data with length = embedLen", () => {
        // prettier-ignore
        let data = new Uint8Array([68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83]);
        let point = curve.point().embed(data, randomBytes);

        let x = new BN(
          "55302791189059782649052548129238946009757254481983766602279814276765761229598",
          10
        );
        let y = new BN(
          "61392340999017759760595650359991412638601309952067335192619881551703613872215",
          10
        );

        let target = new nist.Point(curve, x, y);
        assert.isTrue(point.equal(target), "point != target");
      });
    });

    describe("data", () => {
      it("should extract embedded data", () => {
        let x = new BN(
          "73817101515731741206419437436780703770216083351543551172201261211469822298629",
          10
        );
        let y = new BN(
          "88765585354250601676518938452147013066867582066920092774664806337343312248839",
          10
        );
        let point = new nist.Point(curve, x, y);
        let data = new Uint8Array([2, 4, 6, 8, 10]);
        assert.deepEqual(point.data(), data, "data returned wrong values");
      });

      it("should throw an Error on embeded length > embedLen", () => {
        setSeed(42);
        randomBytes(65);
        // prettier-ignore
        let bytes = new Uint8Array([4, 201, 209, 147, 190, 134, 219, 80, 165, 6, 231, 153, 126, 240, 204, 175, 212, 170, 3, 0, 156, 228, 220, 14, 189, 212, 105, 250, 84, 26, 5, 195, 137, 6, 162, 237, 154, 18, 5, 159, 120, 82, 140, 135, 94, 18, 162, 95, 112, 39, 108, 199, 167, 17, 65, 78, 9, 156, 173, 246, 10, 104, 224, 192, 157]);
        let point = curve.point();
        point.unmarshalBinary(bytes);
        assert.throws(() => {
          point.data();
        }, Error);
      });
    });

    describe("add", () => {
      it("should add two points", () => {
        let x1 = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );
        let y1 = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );

        let x2 = new BN(
          "34573993922878482166947855247172961010057054898826823466355875611648896895957",
          10
        );
        let y2 = new BN(
          "80433135225576650318672668154277196065185839134887563174194682903173809692971",
          10
        );

        let x3 = new BN(
          "29544735125058142535927177955671190517757885143989883513559208755530587788814",
          10
        );
        let y3 = new BN(
          "87130155067619906103393670168292422617898997176123455498428905788640774614716",
          10
        );

        let p1 = new nist.Point(curve, x1, y1);
        let p2 = new nist.Point(curve, x2, y2);
        let p3 = new nist.Point(curve, x3, y3);
        let sum = curve.point().add(p1, p2);
        // a + b = b + a
        let sum2 = curve.point().add(p2, p1);

        assert.isTrue(curve.curve.validate(sum.ref.point), "sum not on curve");
        assert.isTrue(sum.equal(p3), "sum != p3");
        assert.isTrue(
          curve.curve.validate(sum2.ref.point),
          "sum2 not on curve"
        );
        assert.isTrue(sum2.equal(p3), "sum2 != p3");
      });
    });

    describe("sub", () => {
      it("should subtract two points", () => {
        let x1 = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );
        let y1 = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );

        let x2 = new BN(
          "34573993922878482166947855247172961010057054898826823466355875611648896895957",
          10
        );
        let y2 = new BN(
          "80433135225576650318672668154277196065185839134887563174194682903173809692971",
          10
        );

        let x3 = new BN(
          "110976464347423563140926045017315078040251245233675452604569570242275790207423",
          10
        );
        let y3 = new BN(
          "27383322053734574189591792613880900856836445243349135319283759784129102955550",
          10
        );

        let p1 = new nist.Point(curve, x1, y1);
        let p2 = new nist.Point(curve, x2, y2);
        let p3 = new nist.Point(curve, x3, y3);
        let diff = curve.point().sub(p1, p2);

        assert.isTrue(
          curve.curve.validate(diff.ref.point),
          "diff not on curve"
        );
        assert.isTrue(diff.equal(p3), "diff != p3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        let x1 = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );
        let y1 = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );

        let x2 = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );
        let y2 = new BN(
          "62217035136951908988662373505443311820761056907366430051345262373721931928743",
          10
        );

        let p1 = new nist.Point(curve, x1, y1);
        let p2 = new nist.Point(curve, x2, y2);
        let neg = curve.point().neg(p1);

        assert.isTrue(curve.curve.validate(neg.ref.point), "neg not on curve");
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
        let x1 = new BN(
          "110886497124999652792301595882074601258209437127400215444890406848187894132914",
          10
        );
        let y1 = new BN(
          "53575054073404339774035073443964261709325086507923884144188368935145165925208",
          10
        );

        let x2 = new BN(
          "1597317178653134404256011203581619582648667990919174345469188685075376970978",
          10
        );
        let y2 = new BN(
          "35697504603312581621426847792434067144582647671367433296669068730814336637643",
          10
        );

        let p1 = new nist.Point(curve, x1, y1);
        let buf = new Uint8Array([5, 10]);
        let s = curve.scalar().setBytes(buf);
        let prod = curve.point().mul(s, p1);
        let p2 = new nist.Point(curve, x2, y2);

        assert.isTrue(
          curve.curve.validate(prod.ref.point),
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
          curve.curve.validate(threeBase.ref.point),
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
      it("should set the scalar reading bytes from big endian array", () => {
        let bytes = new Uint8Array([2, 4, 8, 10]);
        let s = curve.scalar().setBytes(bytes);
        let target = new BN("0204080a", 16);
        assert.strictEqual(s.ref.arr.fromRed().cmp(target), 0);
      });

      it("should throw TypeError when b is not Uint8Array", () => {
        assert.throws(() => {
          curve.scalar().setBytes(1234);
        }, TypeError);
      });

      it("should reduce to number to mod N", () => {
        // prettier-ignore
        let bytes = new Uint8Array([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        let s = curve.scalar().setBytes(bytes);
        let target = new BN(
          "ffffffff00000000000000004319055258e8617b0c46353d039cdaae",
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
        let target = new Uint8Array([190, 106, 201, 42, 29, 98, 50, 20, 33, 0, 19, 67, 158, 237, 104, 224, 224, 253, 31, 149, 82, 182, 97, 87, 34, 5, 98, 17, 87, 61, 172, 180]);
        let s3 = curve.scalar().setBytes(target);

        assert.isTrue(sum.equal(s3), "sum != s3");
      });
    });

    describe("sub", () => {
      it("should subtract two scalars", () => {
        let b1 = new Uint8Array([1, 2, 3, 4]);
        let b2 = new Uint8Array([5, 6, 7, 8]);
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let diff = curve.scalar().sub(s1, s2);
        // prettier-ignore
        let target = new Uint8Array([255, 255, 255, 255, 0, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255, 188, 230, 250, 173, 167, 23, 158, 132, 243, 185, 202, 194, 248, 95, 33, 77]);
        let s3 = curve.scalar().setBytes(target);
        assert.isTrue(diff.equal(s3), "diff != s3");
      });
    });

    describe("neg", () => {
      it("should negate a point", () => {
        let s1 = curve.scalar().setBytes(b1);
        let neg = curve.scalar().neg(s1);

        // prettier-ignore
        let target = new Uint8Array([154, 39, 145, 231, 128, 248, 52, 6, 49, 85, 200, 164, 158, 16, 33, 96, 146, 236, 120, 242, 154, 155, 254, 225, 166, 156, 209, 20, 34, 240, 40, 166]);
        let s2 = curve.scalar().setBytes(target);

        assert.isTrue(neg.equal(s2), "neg != s2");
      });
    });

    describe("bytes", () => {
      it("should return the bytes in big-endian representation", () => {
        let s1 = curve.scalar().setBytes(b1);
        let bytes = s1.bytes();

        assert.deepEqual(b1, bytes);
      });
    });

    describe("one", () => {
      let one = curve.scalar().one();

      it("should set the scalar to one", () => {
        let bytes = one.bytes();
        let target = new Uint8Array([1]);
        assert.deepEqual(target, bytes);
      });
    });

    describe("mul", () => {
      it("should multiply two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let prod = curve.scalar().mul(s1, s2);

        // prettier-ignore
        let target = new Uint8Array([88, 150, 22, 208, 89, 155, 151, 255, 177, 162, 187, 27, 200, 24, 106, 226, 148, 44, 50, 249, 104, 23, 185, 233, 226, 79, 51, 233, 132, 194, 166, 138]);
        let s3 = curve.scalar().setBytes(target);

        assert.isTrue(prod.equal(s3), "mul != s3");
      });
    });

    describe("div", () => {
      it("should divide two scalars", () => {
        let s1 = curve.scalar().setBytes(b1);
        let s2 = curve.scalar().setBytes(b2);
        let quotient = curve.scalar().div(s1, s2);

        // prettier-ignore
        let target = new Uint8Array([197, 214, 67, 20, 213, 10, 109, 3, 187, 62, 94, 90, 111, 152, 254, 126, 57, 162, 144, 250, 104, 92, 124, 206, 143, 31, 20, 64, 4, 243, 185, 241]);
        let s3 = curve.scalar().setBytes(target);

        assert.isTrue(quotient.equal(s3), "quotient != s3");
      });
    });
    describe("inv", () => {
      it("should compute the inverse modulo n of scalar", () => {
        let s1 = curve.scalar().setBytes(b1);
        let inv = curve.scalar().inv(s1);

        // prettier-ignore
        let target = new Uint8Array([65, 112, 236, 29, 4, 150, 6, 224, 144, 13, 175, 197, 232, 73, 19, 137, 150, 235, 201, 127, 55, 45, 109, 196, 104, 154, 215, 9, 171, 186, 177, 58]);
        let s2 = curve.scalar().setBytes(target);

        assert.isTrue(inv.equal(s2), "inv != s2");
      });
    });

    describe("pick", () => {
      it("should pick a random scalar", () => {
        setSeed(42);
        let s1 = curve.scalar().pick(randomBytes);

        // prettier-ignore
        let bytes = new Uint8Array([225, 208, 244, 196, 143, 183, 151, 13, 179, 199, 181, 81, 189, 241, 253, 227, 46, 34, 167, 212, 41, 112, 79, 126, 170, 10, 139, 193, 110, 187, 30, 231]);
        let target = curve.scalar().setBytes(bytes);

        assert.isTrue(s1.equal(target));
      });
    });

    describe("marshalBinary", () => {
      it("should return the marshalled representation of scalar", () => {
        let s1 = curve.scalar().setBytes(b1);
        let m = s1.marshalBinary();

        assert.deepEqual(m, b1);
      });
    });

    describe("unmarshalBinary", () => {
      it("should convert marshalled representation to scalar", () => {
        let s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        let bytes = s1.bytes();

        assert.deepEqual(bytes, b1);
      });

      it("should throw an error if input is not Uint8Array", () => {
        let s1 = curve.scalar();
        assert.throws(() => {
          s1.unmarshalBinary(123);
        }, TypeError);
      });

      it("should throw an error if input > q", () => {
        let s1 = curve.scalar();
        // prettier-ignore
        let bytes = new Uint8Array([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        assert.throws(() => {
          s1.unmarshalBinary(bytes);
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
        let s1 = curve.scalar().setBytes(b1);
        // prettier-ignore
        let target = "65d86e177f07cbfaceaa375b61efde9f29fa81bb0c7b9fa34d1cf9aed972fcab";
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
        let target = "01";
        assert.strictEqual(s1.string(), target);
      });
    });
  });

  it("should work with NIST CAVP SP 800-56A ECCCDH Primitive Test Vectors", () => {
    // For each test vector calculate Z = privKey * peerPubKey and assert
    // X Coordinate of calcZ == Z
    for (let i = 0; i < nistVectors.length; i++) {
      let testVector = nistVectors[i];
      let X = new BN(testVector.X, 16);
      let Y = new BN(testVector.Y, 16);
      let privKey = new BN(testVector.Private, 16);
      let peerX = new BN(testVector.PeerX, 16);
      let peerY = new BN(testVector.PeerY, 16);
      let Z = Uint8Array.from(new BN(testVector.Z, 16).toArray());

      let key = curve.scalar().setBytes(Uint8Array.from(privKey.toArray()));
      let pubKey = new nist.Point(curve, X, Y);
      let peerPubKey = new nist.Point(curve, peerX, peerY);

      let calcZ = curve.point().mul(key, peerPubKey);

      assert.isTrue(
        curve.curve.validate(pubKey.ref.point),
        "peerPubKey not on curve"
      );
      assert.isTrue(
        curve.curve.validate(peerPubKey.ref.point),
        "peerPubKey not on curve"
      );
      assert.deepEqual(Z, Uint8Array.from(calcZ.ref.point.getX().toArray()));
    }
  });
});
