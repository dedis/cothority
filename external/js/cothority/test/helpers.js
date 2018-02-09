"use strict";

const identity = require("../lib");

function roster(group, keypairs) {
  const identities = [];
  const n = keypairs.length;
  for (var i = 0; i < n; i++) {
    const pub = keypairs[i].pub;
    const addr = "tcp://127.0.0.1:700" + i * 2;
    identities[i] = new identity.ServerIdentity(group, pub, addr);
  }
  return new identity.Roster(identities);
}

function keypairs(group, n) {
  return Array.from(new Array(n), (val, index) => keypair(group));
}

function keypair(group) {
  const key = group.scalar().pick();
  const pub = group.point().mul(key);
  return {
    priv: key,
    pub: pub
  };
}

module.exports = {
  keypair,
  keypairs,
  roster
};
