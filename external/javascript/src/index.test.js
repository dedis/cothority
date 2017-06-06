import ByteBuffer from 'bytebuffer'
import CothorityMessages from './index'

describe('cothority-messages', () => {

  const mockSignatureRequest = {
    server: {
      address: "tcp://78.46.227.60:7770",
      description: "Ineiti's server",
      public: "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106",
      id: "b208d306b34751fcbbb3d4e9c86588d9"
    },
    message: "801f13291565430b69d0c187ca8975adace1b06514b4030f53fd93a4c0add9c3",
    request: "CiCAHxMpFWVDC2nQwYfKiXWtrOGwZRS0Aw9T/ZOkwK3ZwxJgEl4KIOXiPlhTmgnTIR2PoPs0ddSGVeDAbYPpPI5ufRaqh8EGEhCyCNMGs0dR/Luz1OnIZYjZGhd0Y3A6Ly83OC40Ni4yMjcuNjA6Nzc3MCIPSW5laXRpJ3Mgc2VydmVy"
  };

  it('should create a correct signature request', () => {
    expect.assertions(1);

    const message = ByteBuffer.fromHex(mockSignatureRequest.message).buffer;
    const servers = [{
      address: mockSignatureRequest.server.address,
      description: mockSignatureRequest.server.description,
      public: ByteBuffer.fromHex(mockSignatureRequest.server.public).buffer,
      id: ByteBuffer.fromHex(mockSignatureRequest.server.id).buffer
    }];

    const data = CothorityMessages.createSignatureRequest(message, servers);
    expect(ByteBuffer.wrap(data).toBase64()).toBe(mockSignatureRequest.request);
  });

  const mockSignatureResponse = {
    response: "0a206d6af9d5a3856dcf2cb07772bd8abc8d5f2407b4c808c5f850552457aa42a04c1241290376b59c093b5a378c83cbca0011768bb9cf1660c42e59d2294dcc90cbb4e7c1c187092e0934152fd9f66f337c8a957960ab3acabb805449242e71f4500a0efe",
    hash: "6d6af9d5a3856dcf2cb07772bd8abc8d5f2407b4c808c5f850552457aa42a04c",
    signature: "290376b59c093b5a378c83cbca0011768bb9cf1660c42e59d2294dcc90cbb4e7c1c187092e0934152fd9f66f337c8a957960ab3acabb805449242e71f4500a0efe"
  };

  it('should decode a signature response', () => {
    const response = ByteBuffer.fromHex(mockSignatureResponse.response).buffer;
    const decoded = CothorityMessages.decodeSignatureResponse(response);

    expect(ByteBuffer.wrap(decoded.hash).toHex()).toBe(mockSignatureResponse.hash);
    expect(ByteBuffer.wrap(decoded.signature).toHex()).toBe(mockSignatureResponse.signature);
  });

  const mockStatusResponses = [{
    base64: "CpsCCgZTdGF0dXMSkAIKOgoSQXZhaWxhYmxlX1NlcnZpY2VzEiRDb1NpLEd1YXJkLElkZW50aXR5LFNraXBjaGFpbixTdGF0dXMKFAoIVFh" +
    "fYnl0ZXMSCDMwMTc5NDQ3ChQKCFJYX2J5dGVzEgg0MjU5NTczNwoNCgRQb3J0EgU2MjMwNgofCgtEZXNjcmlwdGlvbhIQRGFlaW5hcidzIENvbm9" +
    "kZQoPCghDb25uVHlwZRIDdGNwCg4KB1ZlcnNpb24SAzEuMAodCgZTeXN0ZW0SE2xpbnV4L2FtZDY0L2dvMS43LjQKFgoESG9zdBIOOTUuMTQzLj" +
    "E3Mi4yNDEKHgoGVXB0aW1lEhQ0MTRoMzhtMzcuNjQxMjkzNTM1cxJiCiBYit3B+9nEA4aODQrCAD58dTjQqRVPvbPPdygi8OvIJxIQvtA5xn6rW" +
    "O2N/6E3NV3DfhoadGNwOi8vOTUuMTQzLjE3Mi4yNDE6NjIzMDYiEERhZWluYXIncyBDb25vZGU=",
    description: "Daeinar's Conode",
    public: "588addc1fbd9c403868e0d0ac2003e7c7538d0a9154fbdb3cf772822f0ebc827"
  }];

  it('should decode a status response', () => {
    expect.assertions(mockStatusResponses.length * 4);

    mockStatusResponses.forEach((mock) => {
      const buffer = Uint8Array.from(atob(mock.base64), c => c.charCodeAt(0));

      const response = CothorityMessages.decodeStatusResponse(buffer);

      expect(response.system).toBeDefined();
      expect(response.system.Status.field).toBeDefined();
      expect(response.system.Status.field.Description).toBe(mock.description);

      expect(isBuffersEqual(response.server.public, ByteBuffer.fromHex(mock.public))).toBeTruthy();
    });
  });

  it('should create a random request', () => {
    expect(CothorityMessages.createRandomMessage()).toBeDefined();
  });

  it('should decode a random response', () => {
    const msg = CothorityMessages.encodeMessage('RandomResponse', {
      R: new Uint8Array([]),
      T: {
        nodes: 1,
        groups: 2,
        purpose: 'test',
        time: Date.now()
      }
    });

    const decoded = CothorityMessages.decodeRandomResponse(msg);
    expect(decoded.T.nodes).toBe(1);
    expect(decoded.R).toBeDefined();
    expect(decoded.T.time).toBeDefined();
  });

});

function isBuffersEqual(b1, b2) {
  return ByteBuffer.wrap(b1).toBase64() === ByteBuffer.wrap(b2).toBase64();
}