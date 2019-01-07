package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

public class Mask {
    private final byte[] mask;
    private final Point[] publics;
    private Point aggregate = null;

    Mask(Point[] publics, byte[] mask) throws CothorityCryptoException {
        if (publics.length == 0) {
            throw new CothorityCryptoException("no public keys");
        }
        this.publics = publics;
        this.mask = new byte[(this.publics.length + 7) >> 3];
        this.aggregate = publics[0].getZero();
        for (int i = 0; i < publics.length; i++) {
            byte byt = (byte)(i >> 3);
            byte msk = (byte)(1 << (i&7));
            if (((this.mask[byt] & msk) == 0) && ((mask[byt] & msk) != 0)) {
                this.mask[byt] ^= msk; // flip bit in mask from 0 to 1
                this.aggregate = this.aggregate.add(this.publics[i]);
            }
            if (((this.mask[byt] & msk) != 0) && ((mask[byt] & msk) == 0)) {
                this.mask[byt] ^= msk; // flip bit in mask from 1 to 0
                this.aggregate = this.aggregate.add(this.publics[i].negate());
            }
        }
    }

    public int len() {
        return (this.publics.length + 7) >> 3;
    }

    public Point getAggregate() {
        return this.aggregate;
    }

    public boolean indexEnabled(int i) throws IndexOutOfBoundsException {
        if (i >= this.publics.length) {
            throw new IndexOutOfBoundsException();
        }
        byte byt = (byte)(i >> 3);
        byte msk = (byte)(1 << (i&7));
        return ((this.mask[byt] & msk) != 0);
    }

    public boolean keyEnabled(Point p) throws CothorityCryptoException {
        for (int i = 0; i < this.publics.length; i++) {
            if (this.publics[i].equals(p)) {
                return this.indexEnabled(i);
            }
        }
        throw new CothorityCryptoException("key not found");
    }

    public int countEnabled() {
        // hw is hamming weight
        int hw = 0;
        for (int i = 0; i < this.publics.length; i++) {
            byte byt = (byte)(i >> 3);
            byte msk = (byte)(1 << (i&7));
            if ((this.mask[byt] & msk) != 0) {
                hw++;
            }
        }
        return hw;
    }

    public int countTotal() {
        return this.publics.length;
    }

    @Override
    public String toString() {
        String out = "";
        out += "mask: " + Hex.printHexBinary(this.mask);
        out += "\npublic keys:";
        for (Point p : this.publics) {
            out += "\n" + p.toString();
        }
        out += "\naggregate: ";
        if (this.aggregate == null) {
            out += "null";
        } else {
            out += this.aggregate.toString();
        }
        return out;
    }
}
