import java.security.PublicKey;
import java.util.Date;
import com.moandjiezana.toml;

public class ServerIdentity {
    public String Address;
    public String Description;
    public PublicKey Public;

    public ServerIdentity(String definition){
        Toml toml = new Toml().read(getTomlFile());
        String someValue = toml.getString("someKey");
        Date someDate = toml.getDate("someTable.someDate");
        MyClass myClass = toml.to(MyClass.class);
    }
}
