/**
 * Class PRNG defines a PRNG using a Linear Congruent Generator
 * Javascript doesn't have a seedable PRNG in stdlib and this
 * implementation is to be used for deterministic outputs in tests
 */
export class PRNG {
    constructor(private seed: number) {
        this.setSeed = this.setSeed.bind(this);
        this.genByte = this.genByte.bind(this);
        this.pseudoRandomBytes = this.pseudoRandomBytes.bind(this);
    }

    getSeed(): number {
        return this.seed;
    }

    setSeed(seed) {
        this.seed = seed;
    }

    genByte() {
        this.seed = (this.seed * 9301 + 49297) % 233280;
        let rnd = this.seed / 233280;

        return Math.floor(rnd * 255);
    }

    pseudoRandomBytes(n) {
        const arr = Buffer.alloc(n, 0);
        for (let i = 0; i < n; i++) {
            arr[i] = this.genByte();
        }
        return arr;
    }
}

export function unhexlify(str: string): Buffer {
    const result = Buffer.allocUnsafe(str.length >> 1);
    for (let c = 0, i = 0, l = str.length; i < l; i += 2, c++) {
        result[c] = parseInt(str.substr(i, 2), 16);
    }
    return result;
}

export function hexToBuffer(hex: string): Buffer {
    let bytes = Buffer.allocUnsafe(hex.length >> 1);
    for (let i = 0; i < bytes.length; i++) {
        bytes[i] = parseInt(hex.substr(i << 1, 2), 16);
    }
    return bytes;
};
