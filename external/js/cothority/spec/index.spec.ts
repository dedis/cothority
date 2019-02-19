import { byzcoin } from "../src";

describe("Module import Tests", () => {
    it("should import the module", () => {
        expect(byzcoin.ByzCoinRPC).toBeDefined();
    });
});
