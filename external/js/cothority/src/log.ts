const defaultLvl = 2;
const lvlStr = ["E", "W", "I", "!4", "!3", "!2", "!1", "P", "1", "2", "3", "4"];

const regex = /([^/]+:[0-9]+):[0-9]+/;

export class Logger {
    private _lvl: number;
    private _out: (str: string) => void;

    constructor(lvl: number) {
        this._lvl = lvl === undefined ? defaultLvl : lvl;
        // tslint:disable-next-line
        this._out = console.log;
    }

    set lvl(l) {
        this._lvl = l;
    }

    get lvl(): number {
        return this._lvl;
    }

    set out(fn: (str: string) => void) {
        this._out = fn;
    }

    print(...args: any[]) {
        this.printLvl(0, args);
    }

    lvl1(...args: any[]) {
        this.printLvl(1, args);
    }

    lvl2(...args: any[]) {
        this.printLvl(2, args);
    }

    lvl3(...args: any[]) {
        this.printLvl(3, args);
    }

    lvl4(...args: any[]) {
        this.printLvl(4, args);
    }

    llvl1(...args: any[]) {
        this.printLvl(-1, args);
    }

    llvl2(...args: any[]) {
        this.printLvl(-2, args);
    }

    llvl3(...args: any[]) {
        this.printLvl(-3, args);
    }

    llvl4(...args: any[]) {
        this.printLvl(-4, args);
    }

    info(...args: any[]) {
        this.printLvl(-5, args);
    }

    warn(...args: any[]) {
        this.printLvl(-6, args);
    }

    error(...args: any[]) {
        this.printLvl(-7, args);
    }

    private joinArgs(args: any[]) {
        return args.map((a) => {
            if (typeof a === "string") {
                return a;
            }
            if (a == null) {
                return "null";
            }
            if (a instanceof Error) {
                return `${a.message}\n${a.stack}`;
            }
            if (a instanceof Buffer) {
                return a.toString("hex");
            }
            if (a.toString instanceof Function) {
                const str = a.toString();
                if (str !== "[object Object]") {
                    return str;
                }
            }
            if (a.constructor) {
                return `[Class ${a.constructor.name}]`;
            }

            return `${a}`;
        }).join(" ");
    }

    private printCaller(err: Error, i: number): string {
        const lines = err.stack.split("\n");
        if (lines.length <= i - 1) {
            return "";
        }

        const matches = lines[i - 1].match(regex);

        if (matches && matches.length >= 1) {
            return matches[1];
        }

        return "";
    }

    private printLvl(l: number, args: any[]) {
        let indent = Math.abs(l);
        indent = indent >= 5 ? 0 : indent;
        if (l <= this._lvl) {
            this._out(`[${lvlStr[l + 7]}] ${this.printCaller(new Error(), 4)}: ${this.joinArgs(args)}`);
        }
    }
}

export default new Logger(2);
