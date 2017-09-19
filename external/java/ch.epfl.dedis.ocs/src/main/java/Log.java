import javax.xml.bind.DatatypeConverter;

public class Log {
    public static void Lvl(int l, Object... args){
        System.out.println(args);
    }

    public static String toString(byte[] b){
        return DatatypeConverter.printHexBinary(b);
    }
}
