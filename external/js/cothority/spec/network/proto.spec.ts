/* tslint:disable max-line-length */
import { Roster, ServerIdentity } from "../../src/network/proto";

describe("Network Proto Tests", () => {
    it("should parse a roster", () => {
        const str = `
        [[servers]]
            Address = "tcp://127.0.0.1:7001"
            Public = "4e3008c1a2b6e022fb60b76b834f174911653e9c9b4156cc8845bfb334075655"
            Description = "conode1"
            Suite = "Ed25519"
            [servers.Services]
                [servers.Services.ByzCoin]
                    Public = "593c700babf825b6056a2339ce437f73f717226a77d618a5e8f0251c00273b38557c3cda8dbde5431d062804275f8757a2c942d888ac09f2df34f806e35e660a3c6f13dc64a7cf112865807450ccbd9f75bb3aadb98599f7034cf377a9b976045df374f840e9ee617631257fc9611def6c7c2e5cf23f5ab36cf72f68f14b6686"
                    Suite = "bn256.adapter"
        [[servers]]
            Address = "tcp://127.0.0.1:7003"
            Public = "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106"
            Description = "conode2"
            Suite = "Ed25519"
        `;
        const roster = Roster.fromTOML(str);

        expect(roster.length).toBe(2);
        expect(roster.slice(1, 2).length).toBe(1);
        expect(roster.aggregate.length).toBeGreaterThan(0);
    });

    it("should generate the roster ID", () => {
        const roBytes = "0a10be07d7e9bb4454879b1b61696e390b48129a010a2865642e706f696e749a93c8ddc9b3c7750b2c1b5ff2636aa" +
            "455dd10dca7d2f1e9f26674080ed68d1512400a0b53657276696365546573741207456432353531391a2865642e706f696e74b" +
            "fd2a1a547d750e87c14d6f1c11eedcc0628c43c8ff288421274085a00a501f91a10967578c5e8cc5a81af09eb18466940de221" +
            "66c6f63616c3a2f2f3132372e302e302e313a323030302a003a00129a010a2865642e706f696e74c9fcb5d21be1721b6c8c32d" +
            "f86e89812178556c9e3dbc211d27f3a602a4548fa12400a0b53657276696365546573741207456432353531391a2865642e706" +
            "f696e74a0647d4d217f67ced6084649f4d61a2a7786e6c55da648386b3e0802e97ac7a71a10ce118cfbd568559faa8660f99d4" +
            "ad3eb22166c6f63616c3a2f2f3132372e302e302e313a323030312a003a001a2865642e706f696e74553501567de11b0befd52" +
            "5c4485894c13f2f930958abcfa4bfd8d959951ab217";
        const roster = Roster.decode(Buffer.from(roBytes, "hex"));

        expect(roster.id.toString("hex")).toBe("be07d7e9bb4454879b1b61696e390b48");
    });

    it("should get a websocket address", () => {
        const srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]) });

        expect(srvid.getWebSocketAddress()).toBe("ws://127.0.0.1:5001");
    });

    it("should valid and invalid addresses", () => {
        expect(ServerIdentity.isValidAddress("tls://127.0.0.1:5000")).toBeTruthy();
        expect(ServerIdentity.isValidAddress("tls://127.0.0.1:5000000")).toBeFalsy();
        expect(ServerIdentity.isValidAddress("tcp://127.0.0.1:5000")).toBeFalsy();
        expect(ServerIdentity.isValidAddress("")).toBeFalsy();
    });
});
