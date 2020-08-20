import { Message } from "protobufjs/light";
import { addJSON, registerMessage } from "../../src/protobuf";

class MessageTest extends Message<MessageTest> {
    readonly testField: string;
}

class MessageTest2 extends Message<MessageTest2> {
    readonly testField2: string;
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

        const json2 = {
            nested: {
                TestMessage2: {
                    fields: {
                        testField2: { type: "string", id: 1 },
                    },
                },
            },
        };

        addJSON(json);
        registerMessage("TestMessage", MessageTest);

        const buf = MessageTest.encode(new MessageTest({ testField: "abc" })).finish();
        const msg = MessageTest.decode(buf);
        expect(msg.testField).toBe("abc");

        addJSON(json2);
        registerMessage("TestMessage2", MessageTest2);

        const buf2 = MessageTest2.encode(new MessageTest2({ testField2: "abc" })).finish();
        const msg2 = MessageTest2.decode(buf2);
        expect(msg2.testField2).toBe("abc");
    });
});
