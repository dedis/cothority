import { Buffer } from "buffer/";
import util from "util";

const defaultLvl = 2;

const lvlStr = ["E ", "W ", "I ", "!4", "!3", "!2", "!1", "P ", " 1", " 2", " 3", " 4"];

export class Logger {

    lvl: number;
    stackFrameOffset: number = 0;

    constructor(lvl: number | undefined) {
        this.lvl = lvl === undefined ? defaultLvl : lvl;
    }

    out = (...str: string[]) => {
        // tslint:disable-next-line
        console.log(str.join(" "));
    }

    joinArgs(args: any) {
        return args.map((a: any) => {
            if (typeof a === "string") {
                return a;
            }
            if (a == null) {
                return "null";
            }
            try {
                // return JSON.stringify(a, undefined, 4);
                let type: string = typeof a;
                if (a === Object(a)) {
                    if (a.constructor) {
                        type = a.constructor.name;
                    }
                }

                // Have some special cases for the content
                let content = a.toString();
                if (type === "Uint8Array" || type === "Buffer") {
                    content = Buffer.from(a).toString("hex");
                } else if (content === "[object Object]") {
                    content = util.inspect(a);
                }
                return "{" + type + "}: " + content;
            } catch (e) {
                // tslint:disable-next-line
                this.out("error while inspecting:", e);

                return a;
            }
        }).join(" ");
    }

    printCaller(err: (Error | string), i: number): any {
        try {
            const stack = (err as Error).stack.split("\n");
            const method = stack[i + this.stackFrameOffset].trim().replace(/^at */, "").split("(");
            let module = "unknown";
            let file = method[0].replace(/^.*\//g, "");
            if (method.length > 1) {
                module = method[0];
                file = method[1].replace(/^.*\/|\)$/g, "");
            }

            // @ts-ignore
            return (file).padEnd(30);
        } catch (e) {
            return this.out("Couldn't get stack - " + e.toString(), (i + 2 + this.stackFrameOffset).toString());
        }
    }

    printLvl(l: number, args: any) {
        let indent = Math.abs(l);
        indent = indent >= 5 ? 0 : indent;
        if (l <= this.lvl) {
            // tslint:disable-next-line
            this.out(lvlStr[l + 7] + ": " + this.printCaller(new Error(), 3) +
                " -> " + " ".repeat(indent * 2) + this.joinArgs(args));
        }
    }

    print(...args: any) {
        this.printLvl(0, args);
    }

    lvl1(...args: any) {
        this.printLvl(1, args);
    }

    lvl2(...args: any) {
        this.printLvl(2, args);
    }

    lvl3(...args: any) {
        this.printLvl(3, args);
    }

    lvl4(...args: any) {
        this.printLvl(4, args);
    }

    llvl1(...args: any) {
        this.printLvl(-1, args);
    }

    llvl2(...args: any) {
        this.printLvl(-2, args);
    }

    llvl3(...args: any) {
        this.printLvl(-3, args);
    }

    llvl4(...args: any) {
        this.printLvl(-4, args);
    }

    info(...args: any) {
        this.printLvl(-5, args);
    }

    warn(...args: any) {
        this.printLvl(-6, args);
    }

    error(...args: any) {
        this.printLvl(-7, args);
    }

    catch(e: (Error | string), ...args: any) {
        let errMsg = e;
        if (e instanceof Error) {
            errMsg = e.message;
        }
        if (e instanceof Error) {
            for (let i = this.stackFrameOffset; i < e.stack.split("\n").length - this.stackFrameOffset; i++) {
                this.out("C : " + this.printCaller(e, i) + " -> " + this.joinArgs(args));
            }
        } else {
            this.out("C : " + this.printCaller(new Error(), 2) + " -> (" + errMsg + ")");
        }
    }

    rcatch(e: (Error | string), ...args: any): Promise<any> {
        this.stackFrameOffset++;
        this.catch(e, ...args);
        this.stackFrameOffset--;
        const errMsg = e instanceof Error ? e.message : e;
        return Promise.reject(errMsg.toString().replace(/Error: /, ""));
    }
}

// tslint:disable-next-line
let Log = new Logger(defaultLvl);
export default Log;
