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
            Url = "ws:127.0.0.1:7010"
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

        const str = `
        [[servers]]
        Address = "tls://127.0.0.1:7770"
        Suite = "Ed25519"
        Public = "741b3af1fa069b2b964102bae6bc707315f61f5564fae426c261f5b6ceda3590"
        Description = "Conode_1"
        [servers.Services]
          [servers.Services.ByzCoin]
            Public = "3a2fde872cde581442bd9d522f5d9c0d71a52acc739b3e826e1fef9112cc34d172613965c692465d0f11bf89dbea407c1c34ee6fe9767baaa0314501433e520a7504651c8b321a811ee0b3de86cd03a8b187a6f7b4a1d6f89316b4bfd22bae44738f890bbd608c9145e2e8fc10f11e8f42ba4800171c8d7555418919900d7d9d"
            Suite = "bn256.adapter"
          [servers.Services.Skipchain]
            Public = "55eb6fdb543561dbb806e8357e013e17988ba60e210552d05b178c831c0caaa4241bf16bc3f8bafebf3b81bca839bd1696a45dfc3990f992ac165e132474894003bf57c3761eed667cb2af0f7056daec53619a833a26b446fe0c8762b63ed0f145a8b49f3a92704c21715aef5f3e1b2e5769a069123965df3f20b4310cb73fc0"
            Suite = "bn256.adapter"
        `;
        const roster = Roster.fromTOML(str);

        expect(roster.list[0].getWebSocketAddress()).toBe("ws://127.0.0.1:7771");
    });

    it("getWebSocketAddress should return the correct url if the 'url' field is not empty", () => {
        let url = "http://example.com/path";
        let srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]), url });
        let result = srvid.getWebSocketAddress();
        let expected = "ws://example.com/path";
        expect(result).toBe(expected);

        url = "https://example.com:6000/path";
        srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]), url });
        result = srvid.getWebSocketAddress();
        expected = "wss://example.com:6000/path";
        expect(result).toBe(expected);

        url = "https://example.com:3000/path/";
        srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]), url });
        result = srvid.getWebSocketAddress();
        expected = "wss://example.com:3000/path";
        expect(result).toBe(expected);

        url = "https://example.com:3000/";
        srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]), url });
        result = srvid.getWebSocketAddress();
        expected = "wss://example.com:3000";
        expect(result).toBe(expected);

        url = "tcp://example.com/path";
        srvid = new ServerIdentity({ address: "tls://127.0.0.1:5000", id: Buffer.from([]), url });
        expect( () => { srvid.getWebSocketAddress(); }).toThrow();
    });

    it("should valid and invalid addresses", () => {
        expect(ServerIdentity.isValidAddress("tls://127.0.0.1:5000")).toBeTruthy();
        expect(ServerIdentity.isValidAddress("tls://127.0.0.1:5000000")).toBeFalsy();
        expect(ServerIdentity.isValidAddress("tcp://127.0.0.1:5000")).toBeFalsy();
        expect(ServerIdentity.isValidAddress("")).toBeFalsy();
    });
});
