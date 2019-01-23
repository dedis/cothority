import {Identity} from "~/lib/cothority/darc/Identity";

export class Signature {
  /**
   *
   * @param {Uint8Array} [signature]
   * @param {Identity} signer
   */
  constructor(public signature: Buffer, public signer: Identity) {
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toObject(): object {
    return {
      signature: this.signature,
      signer: this.signer.toObject()
    };
  }
}