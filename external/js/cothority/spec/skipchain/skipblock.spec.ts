import { PointFactory } from "@dedis/kyber";
import { BN256G1Point, BN256G2Point } from "@dedis/kyber/dist/pairing/point";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster, ServerIdentity } from "../../src/network/proto";
import { ByzcoinSignature, ForwardLink, SkipBlock } from "../../src/skipchain/skipblock";

/*
 * TODO
 * Update the hash after https://github.com/dedis/cothority/issues/1701
 */

describe("SkipBlock Tests", () => {
    it("should hash the block", () => {
        const sb = new SkipBlock({
            backlinks: [Buffer.from([1, 2, 3])],
            baseHeight: 4,
            data: Buffer.from([1, 2, 3]),
            genesis: Buffer.from([1, 2, 3]),
            height: 32,
            index: 0,
            maxHeight: 32,
            verifiers: [Buffer.from("a7f6cdb747f856b4aff5ece35a882489", "hex")],
        });

        expect(sb.computeHash().toString("hex"))
            .toBe("698629e47b4736d7c4a0a75c42529e80e0d962eed2e7111b1616ab2ffaab22b5");
    });

    it("should hash the block with a roster", () => {
        const roster = new Roster({
            list: [
                new ServerIdentity({
                    public: Buffer.from(
                        "65642e706f696e7471e96e3fcf50e07c1937ffc3df479f3cd0d5e76f37471439fe556fdc768b225d",
                        "hex",
                    ),
                }),
                new ServerIdentity({
                    public: Buffer.from(
                        "65642e706f696e74510f70f3655f26ec7289a1c23b3fcb258c6fceb546670e3d5fd63ec1020a9ee3",
                        "hex",
                    ),
                }),
            ],
        });

        const sb = new SkipBlock({
            backlinks: [Buffer.from([2])],
            baseHeight: 0,
            data: Buffer.from([3]),
            genesis: Buffer.from([1]),
            height: 0,
            index: 0,
            maxHeight: 0,
            roster,
            verifiers: [Buffer.from("a7f6cdb747f856b4aff5ece35a882489", "hex")],
        });

        expect(sb.computeHash().toString("hex"))
            .toBe("0905c7459d7bd011455160b6faa41d744268794a1733e9d3c45f988204352875");
    });

    it("should hash the forward link", () => {
        const fl = new ForwardLink({
            from: Buffer.from([1, 2, 3]),
            to: Buffer.from([4, 5, 6]),
        });

        expect(fl.hash().toString("hex")).toBe("7192385c3c0605de55bb9476ce1d90748190ecb32a8eed7f5207b30cf6a1fe89");
    });

    it("should hash the forward link with a new roster", () => {
        const fl = new ForwardLink({
            from: Buffer.from([1, 2, 3]),
            newRoster: new Roster({ id: Buffer.alloc(16, 0), aggregate: Buffer.from([]) }),
            to: Buffer.from([4, 5, 6]),
        });

        expect(fl.hash().toString("hex")).toBe("9f86f3f4dcaec1da0c4602ac80a12abf9aca1faf53c8f5920ed5ce2b3708227d");
    });

    it("should not verify wrong forward links", () => {
        const fl = new ForwardLink({
            from: Buffer.from([1, 2, 3]),
            signature: new ByzcoinSignature({ msg: Buffer.allocUnsafe(32), sig: Buffer.allocUnsafe(65) }),
            to: Buffer.from([4, 5, 6]),
        });

        const publics = [new BN256G2Point().pick(), new BN256G2Point().pick()];

        expect(fl.verify(publics).message).toBe("recreated message does not match");

        fl.signature.msg.fill(fl.hash(), 0);
        fl.signature.sig.fill(Buffer.concat([new BN256G1Point().null().marshalBinary(), Buffer.from([1])]));
        expect(fl.verify(publics).message).toBe("signature not verified");
    });
});
