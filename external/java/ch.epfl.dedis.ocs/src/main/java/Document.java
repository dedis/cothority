import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.math.GroupElement;
import proto.OCSProto;

import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;
import javax.xml.bind.DatatypeConverter;
import java.util.Arrays;

public class Document {
    public byte[] id;
    public byte[] data;
    public byte[] extra_data;
    public byte[] symmetricKey;
    public Darc readers;
    public OCSProto.OCSWrite ocswrite;

    public Document(Document doc) {
        id = doc.id;
        data = doc.data;
        extra_data = doc.extra_data;
        symmetricKey = doc.symmetricKey;
        readers = doc.readers;
    }

    public Document(byte[] data, int keylen) {
        this.data = data;
//        symmetricKey = new byte[keylen];
//        new Random().nextBytes(symmetricKey);
        symmetricKey = DatatypeConverter.parseHexBinary("294AEDA9694E0391EEC2D8C133BEBBFF");
        extra_data = "".getBytes();
    }

    public Document(String data, int keylen) {
        this(data.getBytes(), keylen);
    }

    public OCSProto.OCSWrite getWrite(Crypto.Point X) throws Exception {
        if (ocswrite != null) {
            return ocswrite;
        }
        OCSProto.OCSWrite.Builder write = OCSProto.OCSWrite.newBuilder();

        Cipher cipher = Cipher.getInstance(Crypto.algo);
        cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(symmetricKey, Crypto.algoKey));
        byte[] data_enc = cipher.doFinal(data);
        write.setData(ByteString.copyFrom(data_enc));

        Crypto.KeyPair randkp = new Crypto.KeyPair();
        Crypto.Scalar r = randkp.Scalar;
        Crypto.Point U = randkp.Point;
        write.setU(U.toProto());

        Crypto.Point C = X.scalarMult(r);
        for (int from = 0; from < symmetricKey.length; from += Crypto.pubLen) {
            int to = from + Crypto.pubLen;
            if (to > symmetricKey.length) {
                to = symmetricKey.length;
            }
            Crypto.Point keyPoint = Crypto.Point.pubStore(Arrays.copyOfRange(symmetricKey, from, to));
            Crypto.Point Cs = C.add(keyPoint);
            write.addCs(Cs.toProto());
        }
        ocswrite = write.build();
        return write.build();
    }
}
