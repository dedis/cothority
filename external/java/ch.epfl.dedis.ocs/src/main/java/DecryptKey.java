import net.i2p.crypto.eddsa.math.GroupElement;
import proto.OCSProto;

import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;
import javax.xml.bind.DatatypeConverter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public class DecryptKey {
    public List<Crypto.Point> Cs;
    public Crypto.Point XhatEnc;
    public Crypto.Point X;

    public DecryptKey(){
        Cs = new ArrayList<>();
    }

    public DecryptKey(OCSProto.DecryptKeyReply reply) {
        this();
        reply.getCsList().forEach(C -> Cs.add(new Crypto.Point(C)));
        X = new Crypto.Point(reply.getX());
        XhatEnc = new Crypto.Point(reply.getXhatEnc());
    }

    public byte[] decryptDocument(OCSProto.OCSWrite write, Account reader) throws Exception {
        byte[] data_enc = write.getData().toByteArray();
        List<Crypto.Point> Cs = new ArrayList<>();
        write.getCsList().forEach(cs -> Cs.add(new Crypto.Point(cs)));

        // Do some magic
        Crypto.Scalar xc = reader.Scalar.reduce();
        Crypto.Scalar xcInv = xc.negate();
        Crypto.Point XhatDec = xcInv.mul(X);
        Crypto.Point Xhat = XhatEnc.add(XhatDec);
        Crypto.Point XhatInv = Xhat.negate();

        byte[] symmetricKey = "".getBytes();
        for (Crypto.Point C: Cs){
            Crypto.Point keyPointHat = C.add(XhatInv);
            try {
                byte[] keyPart = keyPointHat.pubLoad();
                int lastpos = symmetricKey.length;
                symmetricKey = Arrays.copyOfRange(symmetricKey, 0, symmetricKey.length + keyPart.length);
                System.arraycopy(keyPart, 0, symmetricKey, lastpos, keyPart.length);
            } catch (Crypto.CryptoException c) {
                c.printStackTrace();
                System.out.println("couldn't extract data! " + c.toString());
            }
        }

        Cipher cipher = Cipher.getInstance(Crypto.algo);
        cipher.init(Cipher.DECRYPT_MODE, new SecretKeySpec(symmetricKey, Crypto.algoKey));
        return cipher.doFinal(data_enc);
    }

    public String toString(){
        return String.format("Cs.length: %d\n" +
                "XhatEnc: %bytes\n" +
                "X: %bytes", Cs.size(), XhatEnc, X);
    }
}

    // DecodeKey can be used by the reader of an onchain-secret to convert the
// re-encrypted secret back to a symmetric key that can be used later to
// decode the document.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - Cs - the encrypted key-slices
//   - XhatEnc - the re-encrypted schnorr-commit
//   - xc - the private key of the reader
//
// Output:
//   - key - the re-assembled key
//   - err - an eventual error when trying to recover the data from the points
//    func DecodeKey(suite abstract.Suite, X abstract.Point, Cs[]abstract.Point, XhatEnc abstract.Point,
//                   xc abstract.Scalar) (key[]byte,err error){
//        xcInv:=suite.Scalar().Neg(xc)
//        XhatDec:=suite.Point().Mul(X,xcInv)
//        Xhat:=suite.Point().Add(XhatEnc,XhatDec)
//        XhatInv:=suite.Point().Neg(Xhat)
//
//        // Decrypt Cs to keyPointHat
//        for _,C:=range Cs{
//        keyPointHat:=suite.Point().Add(C,XhatInv)
//        keyPart,err:=keyPointHat.Data()
//        if err!=nil{
//        return nil,err
//        }
//        key=append(key,keyPart...)
//        }
//        return
//        }
