import Docker from "dockerode";
import { byzcoin } from "../src";

describe("Module import Tests", () => {
    it("should import the module", () => {
        expect(byzcoin.ByzCoinRPC).toBeDefined();
    });
});

describe("Docker should be available", () => {
    it("should not yield an error when getting docker infos", async () => {
        const docker = new Docker();
        await expectAsync(docker.info()).toBeResolved();
    });
});
