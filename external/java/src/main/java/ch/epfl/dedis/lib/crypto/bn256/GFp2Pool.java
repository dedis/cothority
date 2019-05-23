package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;
import java.util.LinkedList;

/**
 * This singleton class keeps a pool of GFp. Instead of creating manually new GFp objects that will then
 * be eventually deleted by the GC, one can use this class which will keep no longer used objects and re-use them.
 * If the pool is empty a new object is created. We initialize the GFp2 instance with BigInteger.ZERO
 */
public class GFp2Pool {
    private static GFp2Pool ourInstance = new GFp2Pool();
    private LinkedList<GFp2> gfp2s;
    private int count;

    public static GFp2Pool getInstance() {
        return ourInstance;
    }

    private GFp2Pool() {
        this.gfp2s = new LinkedList<>();
        this.count = 0;
    }

    public GFp2 get() {
        this.count++;
        if(this.gfp2s.isEmpty())
            return new GFp2();

        GFp2 gfp2 = this.gfp2s.pop();
        gfp2.x = BigInteger.ZERO;
        gfp2.y = BigInteger.ZERO;
        return gfp2;
    }

    public void put(GFp2... gfp2s) {
        for(GFp2 gfp2 : gfp2s) {
            this.gfp2s.add(gfp2);
            this.count--;
        }

    }

    public int getCount() { return this.count; }
}
