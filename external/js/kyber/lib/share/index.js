"use strict";

const group = require("../../index.js");

class PriPoly {
    constructor(g, T, secret) {
        if (!(g instanceof group.Group)) {
            throw "first argument must be a suite";
        }
        if (!(secret instanceof group.Scalar)) {
            throw "third argument must be a scalar";
        }
        this.g = g;
        this.T = T;
        this.coeff = new Array(T);
        this.coeff[0] = secret;
        for(let i=1; i<this.coeff.length; i++) {
            this.coeff[i] = g.scalar().pick();
        }
    }

    shares(n) {
        const sh = new Array(n);
        for (let i=0; i < sh.length; i++) {
            sh[i] = this.eval(i);
        }
        return sh;
    }

    eval(i) {
        let xi = this.g.scalar().zero();
        xi.setInt(1 + i);
        let v = this.g.scalar().zero()
        for (let j = this.T - 1; j >= 0; j--) {
            v.mul(v, xi)
            v.add(v, this.coeff[j])
        }
        return new PriShare(i, v);
    }
};

class PriShare {
    constructor(i, v) {
        if (!(typeof(i) == typeof(0))) {
            throw "first argument must be an int";
        }
        if (!(v instanceof group.Scalar)) {
            throw "second argument must be a scalar";
        }
        this.i = i;
        this.v = v;
    }
}

// for each share, if it is in range, then create
// an entry in the returned map: share#->1+share.I
function filterPriShares(g, shares, t, n) {
    let ret = {};
    for (let i=0; i < shares.length; i++) {
        const s = shares[i];
        if (s === undefined || s.i < 0 || s.v === undefined || n <= s.i) {
            continue;
        }
        let ii = g.scalar().zero();
        ii.setInt(1 + s.i);
        ret[i] = ii;
        if (ret.length == t) {
            break;
        }
    }
    return ret;
}

function RecoverSecret(g, shares, t, n) {
    if (!(g instanceof group.Group)) {
        throw "first argument must be a suite";
    }
    if (!(shares instanceof Array)) {
        throw "second argument must be an array";
    }
    if (!(shares[0] instanceof PriShare)) {
        throw "second argument must be an array of PriShare";
    }
    if (!(typeof(t) == typeof(0))) {
        throw "third argument must be an int";
    }
    if (!(typeof(n) == typeof(0))) {
        throw "fourth argument must be an int";
    }

    const x = filterPriShares(g, shares, t, n);
    if (x.length < t) {
        throw "not enough shares to recover secret";
    }

    let acc = g.scalar().zero();
    let num = g.scalar().zero();
    let den = g.scalar().zero();
    let tmp = g.scalar().zero();

    for (let i in x) {
        let xi = x[i];
        num.set(shares[i].v);
        den.one();
        for (let j in x) {
            if (i === j) {
                continue;
            }
            let xj = x[j];
            num.mul(num, xj);
            den.mul(den, tmp.sub(xj, xi));
        }
        acc.add(acc, num.div(num, den));
    };
 	return acc
}

module.exports = {
    PriPoly,
    PriShare,
    RecoverSecret
}