//"use strict";
//var wtf = require("wtfnode");
const chai = require("chai");
const expect = chai.expect;

const cothority = require("../../lib");
const proto = cothority.protobuf;
const skipchain = cothority.skipchain;
const misc = cothority.misc;
const net = cothority.net;
const kyber = require("@dedis/kyber-js");

const helpers = require("../helpers.js");

const curve = new kyber.curve.edwards25519.Curve();
const child_process = require("child_process");

describe.only("skipchain client", () => {
  after(function() {
    //wtf.dump();
    //global.asyncDump();
  });
  it("can retrieve updates from conodes", done => {
    runGolang()
      .then(data => {
        [roster, id] = data;
        //console.log("TEST ROSTER =>");
        //onsole.log(roster);
        //const socket = new net.RosterSocket(roster, "Skipchain");
        const addr1 = roster.identities[0].websocketAddr;
        const socket = new net.Socket(addr1, "Skipchain");
        const requestStr = "GetUpdateChain";
        const responseStr = "GetUpdateChainReply";
        const request = { latestId: misc.hexToUint8Array(id) };
        return socket.send(requestStr, responseStr, request);
      })
      .then(data => {
        console.log("DATA FROM SKIPCHAIN !!!");
        console.log(data);
        expect(true).to.be.true;
        killGolang();
        done();
        console.log("DONE NOW");
      })
      .catch(err => {
        done();
        killGolang();
        throw err;
      });
  });
});

const script_path = "../../../cisc/";
const script_name = "./start_test.sh";
const group_file = "build/public.toml";
const genesis_file = "build/genesis.txt";
const fs = require("fs");

var spawned_conodes;

function killGolang() {
  console.log("KILL");
  spawned_conodes.kill();
  spawned_conodes.stdout.destroy();
  spawned_conodes.stderr.destroy();
  child_process.execSync("pkill go");
}

function runGolang() {
  /*var childProcess = require("child_process");*/
  //var oldSpawn = childProcess.spawn;

  //function mySpawn() {
  //console.log("spawn called");
  //console.log(arguments);
  //var result = oldSpawn.apply(this, arguments);
  //return result;
  //}
  /*const spawn = mySpawn;*/
  const build_dir = process.cwd() + "/test/skipchain/build";
  const spawn = child_process.spawn;
  return new Promise(function(resolve, reject) {
    //console.log("env.PATH = " + process.env.PATH);
    //console.log("env.GOPATH = " + process.env.GOPATH);
    console.log("build_dir = " + build_dir);
    spawned_conodes = spawn("go", ["run", "main.go"], {
      cwd: build_dir,
      env: process.env,
      detached: true
    });
    spawned_conodes.unref();
    console.log("SPAWNED Conode PID: " + spawned_conodes.pid);
    spawned_conodes.on("error", err => {
      console.log("Errrrrooorrrr: " + err);
      throw err;
    });
    spawned_conodes.stdout.setEncoding("utf8");
    spawned_conodes.stdout.on("data", data => {
      if (!data.match(/OK/)) {
        console.log("RECEIVED DATA (NOT OK) => " + data);
        return;
      }
      console.log("RECEIVED DATA (OK) => " + data);
      resolve(data);
    });
    spawned_conodes.on("exit", (code, signal) => {
      console.log("exiting program...");
    });
  }).then(data => {
    // read roster and genesis
    const group_file = build_dir + "/public.toml";
    const genesis_file = build_dir + "/genesis.txt";

    const groupToml = fs.readFileSync(group_file, "utf8");
    const genesisID = fs.readFileSync(genesis_file, "utf8");
    console.log("groupToml:  " + groupToml);
    const roster = cothority.Roster.fromTOML(groupToml);
    return Promise.resolve([roster, genesisID]);
  });
  /*.then(data => {*/
  //return callback(data);
  //})
  //.then(e => {
  //spawned_conodes.kill();
  //})
  //.catch(err => {
  //console.log(err);
  //throw err;
  /*});*/
}

function runConodes() {
  // run the script and wait to see 3 kv pairs written
  const ready = runScript();
  ready.next();
  // read the roster and the genesis id
  const groupToml = fs.readFileSync(group_file, "utf8");
  const genesisID = fs.readFileSync(genesis_file, "utf8");
  console.log("groupToml:  " + groupToml);
  const roster = cothority.Roster.fromTOML(groupToml);
  return [roster, groupToml];
}

function* runScript() {
  console.log("Starting directory: " + process.cwd());
  console.log("script_path = " + script_path);
  process.chdir(script_path);
  console.log("New directory: " + process.cwd());
  child_process.execSync("rm -rf build/cl1");
  child_process.execSync("pkill -9 conode || true");
  const child = spawn("./start_test.sh");
  child.stdout.setEncoding("utf8");
  var nb_kv = 0;
  const expected_kv = 3;
  child.stdout.on("data", chunk => {
    console.log("chunk: " + chunk);
    // data from standard output is here as buffers
    var results = chunk.match(/stored key-value pair/i);
    if (results) {
      nb_kv += results.length;
      console.log("Matched " + results.length + "key value pairs");
      if (nb_kv >= expected_kv) {
        yield;
      }
    }
  });
  child.on("error", err => {
    expect(false).to.be.true;
  });
}
