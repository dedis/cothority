const crypto = require("crypto");
const Request = require("./darc/Request");
const Signer = require("./darc/Signer");
const Signature = require("./darc/Signature");

/**
 * An instruction is sent and executed by OmniLedger.
 */
class Instruction {
  /**
   * Cunstruct an instruction when the complete configuration is known
   *
   * @param {Uint8Array} instanceId - The ID of the object, which must be unique
   * @param {Uint8Array} nonce - the nonce of the object
   * @param {number} index - the index of the instruction in the atomic set
   * @param {number} length - the length of the atomic set
   * @param {Spawn} [spawnInst] - the spawn object, which contains the value and the argument
   * @param {Invoke} [invokeInst] - the invoke object
   * @param {Delete} [deleteInst] - the delete object
   * @param {Signature[]} [signatures] - the list of signatures
   */
  constructor(
    instanceId,
    nonce,
    index,
    length,
    spawnInst,
    invokeInst,
    deleteInst,
    signatures
  ) {
    this._instanceId = instanceId;
    this._nonce = nonce;
    this._index = index;
    this._length = length;
    this._spawnInst = spawnInst;
    this._invokeInst = invokeInst;
    this._deleteInst = deleteInst;
    this._signatures = signatures;
  }

  /**
   * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param {Uint8Array} nonce
   * @param {number} index
   * @param {number} length
   * @param {Spawn} spawnInst
   * @return {Instruction}
   */
  static createSpawnInstruction(nonce, index, length, spawnInst) {
    return new Instruction(
      undefined,
      nonce,
      index,
      length,
      spawnInst,
      undefined,
      undefined,
      undefined
    );
  }

  /**
   * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param {Uint8Array} instanceId
   * @param {Uint8Array} nonce
   * @param {number} index
   * @param {number} length
   * @param {Invoke} invokeInst
   * @return {Instruction}
   */
  static createInvokeInstruction(instanceId, nonce, index, length, invokeInst) {
    return new Instruction(
      instanceId,
      nonce,
      index,
      length,
      undefined,
      invokeInst,
      undefined,
      undefined
    );
  }

  /**
   * Use this constructor if it is a delete instruction, i.e. you want to delete an object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param instanceId
   * @param nonce
   * @param index
   * @param length
   * @param deleteInst
   * @return {Instruction}
   */
  static createDeleteInstruction(instanceId, nonce, index, length, deleteInst) {
    return new Instruction(
      instanceId,
      nonce,
      index,
      length,
      undefined,
      undefined,
      deleteInst,
      undefined
    );
  }

  /**
   * Getter for the instance ID
   *
   * @return {Uint8Array}
   */
  get instanceId() {
    return this._instanceId;
  }

  /**
   * Set the signatures for this instruction
   *
   * @param {Signature[]} sig
   */
  set signatures(sig) {
    this._signatures = sig.slice(0);
  }

  /**
   * This method computes the sha256 hash of the instruction.
   *
   * @return {Uint8Array} the digest
   */
  get hash() {
    const hash = crypto.createHash("sha256");
    hash.update(this._instanceId);
    hash.update(this._nonce);
    hash.update(this.intToArr4(this._index));
    hash.update(this.intToArr4(this._length));
    let args = [];
    if (this._spawnInst !== undefined) {
      hash.update(new Uint8Array([0]));
      hash.update(this._spawnInst.contractId);
      args = this._spawnInst.argumentsList;
    } else if (this._invokeInst !== undefined) {
      hash.update(new Uint8Array([1]));
      args = this._invokeInst.argumentsList;
    } else {
      hash.update(new Uint8Array([2]));
    }
    args.forEach(arg => {
      hash.update(arg.name);
      hash.update(arg.value);
    });

    const b = hash.digest();
    return new Uint8Array(
      b.buffer,
      b.byteOffset,
      b.byteLength / Uint8Array.BYTES_PER_ELEMENT
    );
  }

  /**
   * Outputs the action of the instruction, this action be the same as an action in the corresponding darc. Otherwise
   * this instruction may not be accepted.

   * @return {string} - the action
   */
  get action() {
    let a = "invalid";
    if (this._spawnInst !== undefined) {
      a = "spawn:" + this._spawnInst.contractId;
    } else if (this._invokeInst !== undefined) {
      a = "invoke:" + this._invokeInst.command;
    } else if (this._deleteInst !== undefined) {
      a = "delete";
    }

    return a;
  }

  /**
   *
   * @param {Uint8Array} darcId
   */
  toDarcRequest(darcId) {
    return new Request(
      darcId,
      this.action,
      this.hash,
      this._signatures.map(sig => sig.signer),
      this._signatures.map(sig => sig.signature)
    );
  }

  /**
   * Have a list of signers sign the instruction. The instruction will *not* be accepted by omniledger if it is not
   * signed. The signature will not be valid if the instruction is modified after signing.
   *
   * @param {Uint8Array} darcId
   * @param {Signer[]} signers
   */
  signBy(darcId, signers) {
    this._signatures = [];
    signers.forEach(signer => {
      this._signatures.push(new Signature(undefined, signer.identity));
    });

    const msg = this.toDarcRequest(darcId).hash();
    for (let i = 0; i < this._signatures.length; i++) {
      this._signatures[i] = new Signature(
        signers[i].sign(msg),
        signers[i].identity
      );
    }
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toProtobufValidMessage() {
    let object = {
      instanceid: this._instanceId,
      nonce: this._nonce,
      index: this._index,
      length: this._length,
      signatures: this._signatures.map(sig => sig.toProtobufValidMessage())
    };

    if (this._spawnInst !== undefined) {
      object.spawn = this._spawnInst.toProtobufValidMessage();
    } else if (this._invokeInst !== undefined) {
      object.invoke = this._invokeInst.toProtobufValidMessage();
    } else if (this._deleteInst !== undefined) {
      object.delete = this._deleteInst.toProtobufValidMessage();
    }

    return object;
  }

  /**
   *
   * @param {number} x
   */
  intToArr4(x) {
    let buffer = new ArrayBuffer(4);
    new DataView(buffer).setInt32(0, x, true);

    return new Uint8Array(buffer);
  }
}

module.exports = Instruction;
