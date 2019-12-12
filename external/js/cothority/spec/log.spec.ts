import { Logger } from "../src/log";

class ObjectNoString {}

class ObjectWithString {
    toString(): string {
        return "deadbeef";
    }
}

describe("Logger Tests", () => {
    it("should log correctly", () => {
        const buf: string[] = [];
        const logger = new Logger(3);
        logger.out = (str) => buf.push(str);

        logger.lvl1("a", new ObjectNoString());
        expect(buf[0]).toBe(" 1: log.spec.ts:17:16              ->   a {ObjectNoString}: ObjectNoString {}");

        logger.lvl3("b", new ObjectWithString());
        expect(buf[1]).toBe(" 3: log.spec.ts:20:16              ->       b {ObjectWithString}: deadbeef");

        logger.lvl2("c", 1, "text", true, 0xdeadbeef);
        expect(buf[2]).toContain(" 2: log.spec.ts:23:16              ->     c {number}: 1 text {boolean}: true " +
            "{number}: 3735928559");
    });

    it("should log an error with its stack", () => {
        const error = new Error("deadbeef");
        const buf: string[] = [];
        const logger = new Logger(1);
        logger.out = (str) => buf.push(str);

        logger.catch(error, "abc");
        expect(buf.length).toBeGreaterThan(1);
    });
});
