import { Log } from "../../src";

/**
 * HistoryObs allows a test to wait for a set of changes to occur and to throw an error if a timeout occurs before that.
 * To use it, the `push` method should be called for every new occurrence of the item to be observed.
 * This is usually done in an observer:
 *
 *    const ho = new HistoryObs();
 *    coinInstance.subscribe((c) => ho.push(coinInstance.value.toString()));
 *
 * After that, the test can wait for a number of occurrences on this value:
 *
 *    await h.resolve("0", "100000");
 *
 * This will wait for the history to have at least two elements: "0" and "100000". If during the timeout less than
 * two elements are available, the `resolve` throws an error. It also throws an error if the two first history elements
 * don't correspond to the `resolve` call.
 */
export class HistoryObs {

    private readonly entries: string[] = [];

    constructor(private maxWait = 20) {}

    push(...e: string[]) {
        this.entries.push(...e);
    }

    async resolveInternal(newEntries: string[], complete?: boolean): Promise<void> {
        await expectAsync(this.expect(newEntries, true, complete)).toBeResolved();
    }

    async resolve(...newEntries: string[]): Promise<void> {
        return this.resolveInternal(newEntries);
    }

    async resolveComplete(...newEntries: string[]): Promise<void> {
        return this.resolveInternal(newEntries, true);
    }

    async resolveAll(newEntries: string[]): Promise<void> {
        let found = true;
        while (found) {
            try {
                await this.expect(newEntries, true, false, true);
            } catch (e) {
                Log.lvl4(e);
                found = false;
            }
        }
    }

    async reject(newEntries: string[], complete?: boolean): Promise<void> {
        await expectAsync(this.expect(newEntries, false, complete)).toBeRejected();
    }

    async expect(newEntries: string[], succeed: boolean, complete?: boolean, silent?: boolean): Promise<void> {
        return new Promise(async (res, rej) => {
            try {
                for (let i = 0; i < this.maxWait && this.entries.length < newEntries.length; i++) {
                    if (!silent) {
                        Log.lvl3("waiting", i, this.entries.length, newEntries.length);
                    }
                    await new Promise((resolve) => setTimeout(resolve, 200));
                }
                if (!silent) {
                    if (succeed) {
                        Log.lvl2("History:", this.entries, "wanted:", newEntries);
                    } else {
                        Log.lvl2("Want history:", this.entries, "to fail with:", newEntries);
                    }
                }
                if (this.entries.length < newEntries.length) {
                    throw new Error("not enough entries");
                }
                for (const e of newEntries) {
                    const h = this.entries.splice(0, 1)[0];
                    if (e !== h) {
                        throw new Error(`Got ${h} instead of ${e}`);
                    }
                }
                if (complete && this.entries.length !== 0) {
                    throw new Error(`didn't describe all history: ${this.entries}`);
                }
                res();
            } catch (e) {
                if (succeed) {
                    if (!silent) {
                        Log.error(e);
                    }
                }
                rej(e);
            }
        });
    }
}
