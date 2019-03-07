import { StatusRPC } from "../../src/status";
import { ROSTER, startConodes } from "../support/conondes";

describe("StatusRPC", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    }, 30 * 1000);

    it("should get the status of the conode", async () => {
        const rpc = new StatusRPC(roster);
        rpc.setTimeout(1000);

        expect(roster.length).toBeGreaterThan(0);
        await expectAsync(rpc.getStatus()).toBeResolved();

        for (let i = 1; i < roster.length; i++) {
            await expectAsync(rpc.getStatus(i)).toBeResolved();
        }

        await expectAsync(rpc.getStatus(roster.length)).toBeRejected();
    });
});
