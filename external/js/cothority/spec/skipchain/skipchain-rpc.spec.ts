import { SkipBlock, SkipchainRPC } from "../../src/skipchain";
import { ROSTER, startConodes } from "../support/conondes";

describe("SkipchainRPC Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should create a skipchain and add blocks to it", async () => {
        const rpc = new SkipchainRPC(roster);

        const { latest: genesis } = await rpc.createSkipchain(2, 2);

        for (let i = 0; i < 10; i++) {
            await rpc.addBlock(genesis.hash, Buffer.from("abc"));
        }

        const latest = await rpc.getLatestBlock(genesis.hash);

        expect(latest.index).toBe(10);
        expect(latest.data.toString()).toBe("abc");

        const block = await rpc.getSkipBlock(genesis.hash);
        expect(block.forward.length).toBeGreaterThan(1);

        const { skipblock } = await rpc.getSkipBlockByIndex(genesis.hash, 0);
        expect(skipblock.hash).toEqual(block.hash);

        const update = await rpc.getLatestBlock(genesis.hash);
        expect(update.hash).toEqual(latest.hash);

        const chain = await rpc.getUpdateChain(genesis.hash);
        expect(chain.length).toBe(6);

        const chainIDs = await rpc.getAllSkipChainIDs();
        expect(chainIDs).toContain(genesis.hash);
    });

    it("should create a chain with different roster", async () => {
        const rpc = new SkipchainRPC(roster);
        const rpc2 = new SkipchainRPC(roster.slice(0, 2));
        const { latest: genesis } = await rpc.createSkipchain(1, 1);

        for (let i = 0; i < 3; i++) {
            await rpc.addBlock(genesis.hash, Buffer.from("abc"));
            await rpc2.addBlock(genesis.hash, Buffer.from("def"));
        }

        const chain = await rpc.getUpdateChain(genesis.hash);
        expect(chain.length).toBe(7);
    });

    it("should fail to get the block", async () => {
        const rpc = new SkipchainRPC(roster);

        await expectAsync(rpc.getSkipBlock(Buffer.from([1, 2, 3])))
            .toBeRejectedWith("No such block :: No such block :: No such block :: No such block");

        await expectAsync(rpc.getLatestBlock(Buffer.from([1, 2, 3]))).toBeRejected("Couldn't find latest skipblock");
    });

    it("should verify the chain", async () => {
        const rpc = new SkipchainRPC(roster);
        const blocks: SkipBlock[] = [];

        expect(rpc.verifyChain(blocks).message).toContain("no block");

        const { latest: genesis } = await rpc.createSkipchain();

        for (let i = 0; i < 3; i++) {
            await rpc.addBlock(genesis.hash, Buffer.from("abc"));
        }

        const chain = await rpc.getUpdateChain(genesis.hash);
        expect(rpc.verifyChain(chain)).toBeNull();

        const chainErr = chain.slice();
        chainErr.push(new SkipBlock());
        expect(rpc.verifyChain(chainErr).message).toContain("invalid block hash");

        chainErr.splice(3, 1, chainErr[1]);
        expect(rpc.verifyChain(chainErr).message).toContain("no forward link");
    });
});
