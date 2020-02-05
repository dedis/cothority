interface Indy {
  createWallet(config: {}, credentials: {}): Promise<void>;
  openWallet(config: {}, credentials: {}): Promise<WalletHandle>;
  closeWallet(wh: WalletHandle): Promise<void>;
  deleteWallet(config: {}, credentials: {}): Promise<void>;
  createAndStoreMyDid(
    wh: WalletHandle,
    credentials: {}
  ): Promise<[Did, Verkey]>;
  keyForLocalDid(wh: WalletHandle, did: Did): Promise<Verkey>;
  keyForDid(ph: PoolHandle, wh: WalletHandle, did: Did): Promise<Verkey>;
  cryptoAnonCrypt(recipientVk: Verkey, messageRaw: Buffer): Promise<Buffer>;
  cryptoSign(
    wh: WalletHandle,
    signerVk: Verkey,
    messageRaw: Buffer
  ): Promise<Buffer>;
  cryptoVerify(
    signerVk: Verkey,
    messageRaw: Buffer,
    signatureRaw: Buffer
  ): Promise<boolean>;
  createKey(wh: WalletHandle, key: KeyConfig): Promise<Verkey>;
  packMessage(
    wh: WalletHandle,
    message: Buffer,
    receiverKeys: Verkey[],
    senderVk: Verkey | null
  ): Promise<Buffer>;
  unpackMessage(wh: WalletHandle, jwe: Buffer): Promise<Buffer>;
  addWalletRecord(
    wh: WalletHandle,
    type: string,
    id: string,
    value: string,
    tags: {}
  ): Promise<void>;
  updateWalletRecordValue(
    wh: WalletHandle,
    type: string,
    id: string,
    value: string
  ): Promise<void>;
  updateWalletRecordTags(
    wh: WalletHandle,
    type: string,
    id: string,
    tags: {}
  ): Promise<void>;
  addWalletRecordTags(
    wh: WalletHandle,
    type: string,
    id: string,
    tags: {}
  ): Promise<void>;
  deleteWalletRecord(wh: WalletHandle, type: string, id: string): Promise<void>;
  getWalletRecord(
    wh: WalletHandle,
    type: string,
    id: string,
    options: {}
  ): Promise<WalletRecord>;
  openWalletSearch(
    wh: WalletHandle,
    type: string,
    query: {},
    options: {}
  ): Promise<SearchHandle>;
  fetchWalletSearchNextRecords(
    wh: WalletHandle,
    searchHandle: SearchHandle,
    count: number
  ): Promise<WalletRecordSearch>;
  closeWalletSearch(sh: SearchHandle): Promise<void>;
}

declare module "indy-sdk" {
  function createWallet(config: {}, credentials: {}): Promise<void>;
  function openWallet(config: {}, credentials: {}): Promise<WalletHandle>;
  function closeWallet(wh: WalletHandle): Promise<void>;
  function deleteWallet(config: {}, credentials: {}): Promise<void>;
  function createAndStoreMyDid(
    wh: WalletHandle,
    credentials: {}
  ): Promise<[Did, Verkey]>;
  function keyForLocalDid(wh: WalletHandle, did: Did): Promise<Verkey>;
  function cryptoAnonCrypt(
    recipientVk: Verkey,
    messageRaw: Buffer
  ): Promise<Buffer>;
  function cryptoSign(
    wh: WalletHandle,
    signerVk: Verkey,
    messageRaw: Buffer
  ): Promise<Buffer>;
  function cryptoVerify(
    signerVk: Verkey,
    messageRaw: Buffer,
    signatureRaw: Buffer
  ): Promise<boolean>;
  function createKey(wh: WalletHandle, key: KeyConfig): Promise<Verkey>;
  function packMessage(
    wh: WalletHandle,
    message: Buffer,
    receiverKeys: Verkey[],
    senderVk: Verkey | null
  ): Promise<Buffer>;
  function unpackMessage(wh: WalletHandle, jwe: Buffer): Promise<Buffer>;
  function addWalletRecord(
    wh: WalletHandle,
    type: string,
    id: string,
    value: string,
    tags: {}
  ): Promise<void>;
  function updateWalletRecordValue(
    wh: WalletHandle,
    type: string,
    id: string,
    value: string
  ): Promise<void>;
  function updateWalletRecordTags(
    wh: WalletHandle,
    type: string,
    id: string,
    tags: {}
  ): Promise<void>;
  function addWalletRecordTags(
    wh: WalletHandle,
    type: string,
    id: string,
    tags: {}
  ): Promise<void>;
  function deleteWalletRecord(
    wh: WalletHandle,
    type: string,
    id: string
  ): Promise<void>;
  function getWalletRecord(
    wh: WalletHandle,
    type: string,
    id: string,
    options: {}
  ): Promise<WalletRecord>;
  function openWalletSearch(
    wh: WalletHandle,
    type: string,
    query: {},
    options: {}
  ): Promise<SearchHandle>;
  function fetchWalletSearchNextRecords(
    wh: WalletHandle,
    searchHandle: SearchHandle,
    count: number
  ): Promise<WalletRecordSearch>;
  function closeWalletSearch(sh: SearchHandle): Promise<void>;
}

type PoolHandle = number;
type WalletHandle = number;
type SearchHandle = number;
type Did = string;
type Verkey = string;
type ByteArray = number[];

interface KeyConfig {
  seed?: string;
}

interface WalletRecord {
  id: string;
  type?: string;
  value?: string;
  tags?: {};
}

interface WalletRecordSearch {
  totalCount: string | null;
  records: WalletRecord[];
}
