import { Message } from "protobufjs/light";
import { addJSON, registerMessage } from "../../src/protobuf";

class MessageTest extends Message<MessageTest> {
    readonly testField: string;
}

describe("Protobuf Tests", () => {
    it("should add the new model to the root", () => {
        const json = {
            nested: {
                TestMessage: {
                    fields: {
                        testField: { type: "string", id: 1 },
                    },
                },
            },
        };

        addJSON(json);
        registerMessage("TestMessage", MessageTest);

        const buf = MessageTest.encode(new MessageTest({ testField: "abc" })).finish();
        const msg = MessageTest.decode(buf);
        expect(msg.testField).toBe("abc");
    });
});
