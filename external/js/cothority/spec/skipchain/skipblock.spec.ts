import { BN256G1Point, BN256G2Point } from "@dedis/kyber/pairing/point";
import { Roster, ServerIdentity } from "../../src/network/proto";
import { ByzcoinSignature, ForwardLink, SkipBlock } from "../../src/skipchain/skipblock";

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
            .toBe("36a9ae78a58ea8a1dd7f851a7c0d163d7456f016eed30e9e41db1a80f017bcc0");
    });

    it("should hash the block with a roster", () => {
        const roster = new Roster({
            list: [
                new ServerIdentity({
                    public: Buffer.from(
                        "65642e706f696e748d463370bcf61e31b64bdf06a7c2bbb752e09f6bcee2396847200fd74539ac4f",
                        "hex",
                    ),
                }),
                new ServerIdentity({
                    public: Buffer.from(
                        "65642e706f696e74770155e2439b99be3407301a43d41a4cd0f3222ac7db0411696bcccf0e9f2990",
                        "hex",
                    ),
                }),
            ],
        });

        const sb = new SkipBlock({
            backlinks: [Buffer.from([1, 2, 3])],
            baseHeight: 4,
            data: Buffer.from([1, 2, 3]),
            genesis: Buffer.from([1, 2, 3]),
            height: 32,
            index: 0,
            maxHeight: 32,
            roster,
            verifiers: [Buffer.from("a7f6cdb747f856b4aff5ece35a882489", "hex")],
        });

        expect(sb.computeHash().toString("hex"))
            .toBe("bdbe534e525441980184bb53692da069a7ae9ecc5cafcc4f64cb54fc453ff02b");
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
