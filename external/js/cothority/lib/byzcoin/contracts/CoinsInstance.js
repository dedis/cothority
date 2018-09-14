const Instance = require("../Instance");
const Invoke = require("../Invoke");
const root = require("../../protobuf").root;
const Argument = require("../Argument");
const Instruction = require("../Instruction");
const ClientTransaction = require("../ClientTransaction");

class CoinsInstance {
  /**
   * Creates a new CoinsInstance
   * @param {ByzCoinRPC} bc - the ByzCoinRPC instance
   * @param {Uint8Array} instanceId - id of the instance
   * @param {Instance} [instance] - the complete instance
   * @param {string} [type] - the type of coin
   * @param {number} [balance] - the current balance of the account
   */
  constructor(bc, instanceId, instance, type, balance) {
    this._bc = bc;
    this._instanceId = instanceId;
    this._instance = instance;
    this._type = type;
    this._balance = balance;
  }

  /**
   * Getter for the type
   * @return {string}
   */
  get type() {
    return this._type;
  }

  /**
   * Getter for the balance
   * @return {number}
   */
  get balance() {
    return this._balance;
  }

  /**
   * Creates a new instance of CoinsInstance and contact the  byzcoin to try
   * to update the data
   *
   * @param {ByzCoinRPC} bc - the byzcoin instance
   * @param {Uint8Array} instanceId - the instance ID of the contract instance
   * @return {Promise<CoinsInstance>} - a promise that complete when the data
   * have been updated
   */
  static fromInstanceId(bc, instanceId) {
    return new CoinsInstance(bc, instanceId).update();
  }

  /**
   * Transfer a certain amount of coin to another account.
   *
   * @param {number} coins - the amount
   * @param {Uint8Array} to - the destination account (must be a coin contract instace id)
   * @param {Signer} signer - the signer (of the giver account)
   * @return {Promise} - a promisse that completes once the transaction has been
   * included in the ledger.
   */
  transfer(coins, to, signer) {
    let args = [];
    let buffer = new ArrayBuffer(8);
    new DataView(buffer).setBigUint64(0, BigInt(coins), true);

    args.push(new Argument("coins", new Uint8Array(buffer)));
    args.push(new Argument("destination", to));

    let invoke = new Invoke("transfer", args);
    let inst = Instruction.createInvokeInstruction(
      this._instanceId,
      new Uint8Array(32),
      0,
      1,
      invoke
    );

    inst.signBy(this._instance.darcId, [signer]);
    const trans = new ClientTransaction([inst]);

    return this._bc.sendTransactionAndWait(trans, 10);
  }

  /**
   * Update the data of this instance
   *
   * @return {Promise<CoinsInstance>} - a promise that resolves once the data
   * are up-to-date
   */
  update() {
    return this._bc.getProof(this._instanceId).then(proof => {
      this._instance = Instance.fromProof(proof);
      const model = root.lookup("CoinInstance");
      const protoObject = model.decode(this._instance.data);

      this._type = Array.from(protoObject.type)
        .map(c => String.fromCharCode(c))
        .join("");
      this._balance = protoObject.balance;

      return Promise.resolve(this);
    });
  }
}

module.exports = CoinsInstance;
