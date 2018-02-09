## Modules

<dl>
<dt><a href="#module_constants">constants</a></dt>
<dd></dd>
<dt><a href="#module_curves/edwards25519/curve">curves/edwards25519/curve</a></dt>
<dd></dd>
<dt><a href="#module_curves/edwards25519/point">curves/edwards25519/point</a></dt>
<dd></dd>
<dt><a href="#module_curves/edwards25519/scalar">curves/edwards25519/scalar</a></dt>
<dd></dd>
<dt><a href="#module_curves/nist/curve">curves/nist/curve</a></dt>
<dd></dd>
<dt><a href="#module_curves/nist/point">curves/nist/point</a></dt>
<dd></dd>
<dt><a href="#module_curves/nist/scalar">curves/nist/scalar</a></dt>
<dd></dd>
<dt><a href="#module_sign/schnorr">sign/schnorr</a></dt>
<dd></dd>
</dl>

## Classes

<dl>
<dt><a href="#Group">Group</a></dt>
<dd><p>Group is an abstract class for curves</p>
</dd>
<dt><a href="#Point">Point</a></dt>
<dd><p>Point is an abstract class for representing
a point on an elliptic curve</p>
</dd>
<dt><a href="#Scalar">Scalar</a></dt>
<dd><p>Scalar is an abstract class for representing a scalar
to be used in elliptic curve operations</p>
</dd>
</dl>

## Functions

<dl>
<dt><a href="#bits">bits(bitlen, exact, callback)</a> ⇒ <code>Uint8Array</code></dt>
<dd><p>bits choses a random Uint8Array with a maximum bitlength
If exact is <code>true</code>, chose Uint8Array with <em>exactly</em> that bitlenght not less</p>
</dd>
<dt><a href="#int">int(mod, callback)</a> ⇒ <code>Uint8Array</code></dt>
<dd><p>int choses a random uniform Uint8Array less than given modulus</p>
</dd>
</dl>

<a name="module_constants"></a>

## constants
<a name="module_constants..constants"></a>

### constants~constants : <code>Object</code>
Constants

**Kind**: inner typedef of [<code>constants</code>](#module_constants)  
**Properties**

| Name | Type | Description |
| --- | --- | --- |
| zeroBN | <code>BN.jsObject</code> | BN.js object representing 0 |

<a name="module_curves/edwards25519/curve"></a>

## curves/edwards25519/curve

* [curves/edwards25519/curve](#module_curves/edwards25519/curve)
    * [~Edwards25519](#module_curves/edwards25519/curve..Edwards25519)
        * [.string()](#module_curves/edwards25519/curve..Edwards25519+string) ⇒ <code>string</code>
        * [.scalarLen()](#module_curves/edwards25519/curve..Edwards25519+scalarLen) ⇒ <code>number</code>
        * [.scalar()](#module_curves/edwards25519/curve..Edwards25519+scalar) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.pointLen()](#module_curves/edwards25519/curve..Edwards25519+pointLen) ⇒ <code>number</code>
        * [.point()](#module_curves/edwards25519/curve..Edwards25519+point) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.newKey()](#module_curves/edwards25519/curve..Edwards25519+newKey) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)

<a name="module_curves/edwards25519/curve..Edwards25519"></a>

### curves/edwards25519/curve~Edwards25519
Represents an Ed25519 curve

**Kind**: inner class of [<code>curves/edwards25519/curve</code>](#module_curves/edwards25519/curve)  

* [~Edwards25519](#module_curves/edwards25519/curve..Edwards25519)
    * [.string()](#module_curves/edwards25519/curve..Edwards25519+string) ⇒ <code>string</code>
    * [.scalarLen()](#module_curves/edwards25519/curve..Edwards25519+scalarLen) ⇒ <code>number</code>
    * [.scalar()](#module_curves/edwards25519/curve..Edwards25519+scalar) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.pointLen()](#module_curves/edwards25519/curve..Edwards25519+pointLen) ⇒ <code>number</code>
    * [.point()](#module_curves/edwards25519/curve..Edwards25519+point) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.newKey()](#module_curves/edwards25519/curve..Edwards25519+newKey) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)

<a name="module_curves/edwards25519/curve..Edwards25519+string"></a>

#### edwards25519.string() ⇒ <code>string</code>
Return the name of the curve

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/curve..Edwards25519+scalarLen"></a>

#### edwards25519.scalarLen() ⇒ <code>number</code>
Returns 32, the size in bytes of a Scalar on Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/curve..Edwards25519+scalar"></a>

#### edwards25519.scalar() ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Returns a new Scalar for the prime-order subgroup of Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/curve..Edwards25519+pointLen"></a>

#### edwards25519.pointLen() ⇒ <code>number</code>
Returns 32, the size of a Point on Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/curve..Edwards25519+point"></a>

#### edwards25519.point() ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Creates a new point on the Ed25519 curve

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/curve..Edwards25519+newKey"></a>

#### edwards25519.newKey() ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
it to be a multiple of 8).

**Kind**: instance method of [<code>Edwards25519</code>](#module_curves/edwards25519/curve..Edwards25519)  
<a name="module_curves/edwards25519/point"></a>

## curves/edwards25519/point

* [curves/edwards25519/point](#module_curves/edwards25519/point)
    * [~Point](#module_curves/edwards25519/point..Point)
        * [new Point(curve, X, Y, Z, T)](#new_module_curves/edwards25519/point..Point_new)
        * [.toString()](#module_curves/edwards25519/point..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_curves/edwards25519/point..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_curves/edwards25519/point..Point+set) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.clone()](#module_curves/edwards25519/point..Point+clone) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.null()](#module_curves/edwards25519/point..Point+null) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.base()](#module_curves/edwards25519/point..Point+base) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.embedLen()](#module_curves/edwards25519/point..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, callback)](#module_curves/edwards25519/point..Point+embed) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.data()](#module_curves/edwards25519/point..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_curves/edwards25519/point..Point+add) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.sub(p1, p2)](#module_curves/edwards25519/point..Point+sub) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.neg(p)](#module_curves/edwards25519/point..Point+neg) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.mul(s, [p])](#module_curves/edwards25519/point..Point+mul) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.pick(callback)](#module_curves/edwards25519/point..Point+pick) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
        * [.marshalBinary()](#module_curves/edwards25519/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_curves/edwards25519/point..Point+unmarshalBinary) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)

<a name="module_curves/edwards25519/point..Point"></a>

### curves/edwards25519/point~Point
Represents a Point on the twisted edwards curve
(X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT

The value of the parameters is expcurveted in little endian form if being
passed as a Uint8Array

**Kind**: inner class of [<code>curves/edwards25519/point</code>](#module_curves/edwards25519/point)  

* [~Point](#module_curves/edwards25519/point..Point)
    * [new Point(curve, X, Y, Z, T)](#new_module_curves/edwards25519/point..Point_new)
    * [.toString()](#module_curves/edwards25519/point..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_curves/edwards25519/point..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_curves/edwards25519/point..Point+set) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.clone()](#module_curves/edwards25519/point..Point+clone) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.null()](#module_curves/edwards25519/point..Point+null) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.base()](#module_curves/edwards25519/point..Point+base) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.embedLen()](#module_curves/edwards25519/point..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, callback)](#module_curves/edwards25519/point..Point+embed) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.data()](#module_curves/edwards25519/point..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_curves/edwards25519/point..Point+add) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.sub(p1, p2)](#module_curves/edwards25519/point..Point+sub) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.neg(p)](#module_curves/edwards25519/point..Point+neg) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.mul(s, [p])](#module_curves/edwards25519/point..Point+mul) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.pick(callback)](#module_curves/edwards25519/point..Point+pick) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
    * [.marshalBinary()](#module_curves/edwards25519/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_curves/edwards25519/point..Point+unmarshalBinary) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)

<a name="new_module_curves/edwards25519/point..Point_new"></a>

#### new Point(curve, X, Y, Z, T)

| Param | Type |
| --- | --- |
| curve | <code>module:curves/edwards25519~Edwards25519</code> | 
| X | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| Y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| Z | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 
| T | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObjcurvet</code> | 

<a name="module_curves/edwards25519/point..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
<a name="module_curves/edwards25519/point..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point to compare |

<a name="module_curves/edwards25519/point..Point+set"></a>

#### point.set(p2) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point |

<a name="module_curves/edwards25519/point..Point+clone"></a>

#### point.clone() ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/edwards25519/point..Point) - new Point module:curves/edwards25519/point~Point  
<a name="module_curves/edwards25519/point..Point+null"></a>

#### point.null() ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Set to the neutral element, which is (0, 1) for twisted Edwards
Curve

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
<a name="module_curves/edwards25519/point..Point+base"></a>

#### point.base() ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Set to the standard base point for this curve

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
<a name="module_curves/edwards25519/point..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
<a name="module_curves/edwards25519/point..Point+embed"></a>

#### point.embed(data, callback) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | to embed with length <= embedLen |
| callback | <code>function</code> | to generate a random byte array of given length |

<a name="module_curves/edwards25519/point..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_curves/edwards25519/point..Point+add"></a>

#### point.add(p1, p2) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Returns the sum of two points on the curve

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/edwards25519/point..Point) - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point, addend |
| p2 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point, addend |

<a name="module_curves/edwards25519/point..Point+sub"></a>

#### point.sub(p1, p2) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Subtract two points

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/edwards25519/point..Point) - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point |
| p2 | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point module:curves/edwards25519/point~Point |

<a name="module_curves/edwards25519/point..Point+neg"></a>

#### point.neg(p) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Finds the negative of a point p
For Edwards Curves, the negative of (x, y) is (-x, y)

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/edwards25519/point..Point) - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point to negate |

<a name="module_curves/edwards25519/point..Point+mul"></a>

#### point.mul(s, [p]) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Multiply point p by scalar s

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| s | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Scalar |
| [p] | [<code>Point</code>](#module_curves/edwards25519/point..Point) | Point |

<a name="module_curves/edwards25519/point..Point+pick"></a>

#### point.pick(callback) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | to generate a random byte array of given length |

<a name="module_curves/edwards25519/point..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
Convert a ed25519 curve point into a byte representation

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_curves/edwards25519/point..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes) ⇒ [<code>Point</code>](#module_curves/edwards25519/point..Point)
Convert a Uint8Array back to a ed25519 curve point
[tools.ietf.org/html/rfc8032#scurvetion-5.1.3](tools.ietf.org/html/rfc8032#scurvetion-5.1.3)

**Kind**: instance method of [<code>Point</code>](#module_curves/edwards25519/point..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_curves/edwards25519/scalar"></a>

## curves/edwards25519/scalar

* [curves/edwards25519/scalar](#module_curves/edwards25519/scalar)
    * [~Scalar](#module_curves/edwards25519/scalar..Scalar)
        * [.equal(s2)](#module_curves/edwards25519/scalar..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_curves/edwards25519/scalar..Scalar+set) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.clone()](#module_curves/edwards25519/scalar..Scalar+clone) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.zero()](#module_curves/edwards25519/scalar..Scalar+zero) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.add(s1, s2)](#module_curves/edwards25519/scalar..Scalar+add) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.sub(s1, s2)](#module_curves/edwards25519/scalar..Scalar+sub) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.neg(a)](#module_curves/edwards25519/scalar..Scalar+neg) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.one()](#module_curves/edwards25519/scalar..Scalar+one) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.mul(s1, s2)](#module_curves/edwards25519/scalar..Scalar+mul) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.div(s1, s2)](#module_curves/edwards25519/scalar..Scalar+div) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.inv(a)](#module_curves/edwards25519/scalar..Scalar+inv) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.setBytes(b)](#module_curves/edwards25519/scalar..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.bytes()](#module_curves/edwards25519/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick(callback)](#module_curves/edwards25519/scalar..Scalar+pick) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
        * [.marshalBinary()](#module_curves/edwards25519/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_curves/edwards25519/scalar..Scalar+unmarshalBinary)

<a name="module_curves/edwards25519/scalar..Scalar"></a>

### curves/edwards25519/scalar~Scalar
Scalar represents a value in GF(2^252 + 27742317777372353535851937790883648493)

**Kind**: inner class of [<code>curves/edwards25519/scalar</code>](#module_curves/edwards25519/scalar)  

* [~Scalar](#module_curves/edwards25519/scalar..Scalar)
    * [.equal(s2)](#module_curves/edwards25519/scalar..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_curves/edwards25519/scalar..Scalar+set) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.clone()](#module_curves/edwards25519/scalar..Scalar+clone) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.zero()](#module_curves/edwards25519/scalar..Scalar+zero) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.add(s1, s2)](#module_curves/edwards25519/scalar..Scalar+add) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.sub(s1, s2)](#module_curves/edwards25519/scalar..Scalar+sub) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.neg(a)](#module_curves/edwards25519/scalar..Scalar+neg) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.one()](#module_curves/edwards25519/scalar..Scalar+one) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.mul(s1, s2)](#module_curves/edwards25519/scalar..Scalar+mul) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.div(s1, s2)](#module_curves/edwards25519/scalar..Scalar+div) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.inv(a)](#module_curves/edwards25519/scalar..Scalar+inv) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.setBytes(b)](#module_curves/edwards25519/scalar..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.bytes()](#module_curves/edwards25519/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick(callback)](#module_curves/edwards25519/scalar..Scalar+pick) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
    * [.marshalBinary()](#module_curves/edwards25519/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_curves/edwards25519/scalar..Scalar+unmarshalBinary)

<a name="module_curves/edwards25519/scalar..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |

<a name="module_curves/edwards25519/scalar..Scalar+set"></a>

#### scalar.set(a) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |

<a name="module_curves/edwards25519/scalar..Scalar+clone"></a>

#### scalar.clone() ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
<a name="module_curves/edwards25519/scalar..Scalar+zero"></a>

#### scalar.zero() ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
<a name="module_curves/edwards25519/scalar..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
**Returns**: [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |

<a name="module_curves/edwards25519/scalar..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
**Returns**: [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |

<a name="module_curves/edwards25519/scalar..Scalar+neg"></a>

#### scalar.neg(a) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | Scalar |

<a name="module_curves/edwards25519/scalar..Scalar+one"></a>

#### scalar.one() ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
<a name="module_curves/edwards25519/scalar..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | 
| s2 | [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar) | 

<a name="module_curves/edwards25519/scalar..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param |
| --- |
| s1 | 
| s2 | 

<a name="module_curves/edwards25519/scalar..Scalar+inv"></a>

#### scalar.inv(a) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param |
| --- |
| a | 

<a name="module_curves/edwards25519/scalar..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Sets the scalar from little-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type | Description |
| --- | --- | --- |
| b | <code>Uint8Array</code> | bytes |

<a name="module_curves/edwards25519/scalar..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
<a name="module_curves/edwards25519/scalar..Scalar+pick"></a>

#### scalar.pick(callback) ⇒ [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)
Set to a random scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | to generate random byte array of given length |

<a name="module_curves/edwards25519/scalar..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (little endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  
<a name="module_curves/edwards25519/scalar..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes)
Reads the binary representation (little endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/edwards25519/scalar..Scalar)  

| Param |
| --- |
| bytes | 

<a name="module_curves/nist/curve"></a>

## curves/nist/curve

* [curves/nist/curve](#module_curves/nist/curve)
    * [~Weierstrass](#module_curves/nist/curve..Weierstrass)
        * [new Weierstrass(config)](#new_module_curves/nist/curve..Weierstrass_new)
        * [.string()](#module_curves/nist/curve..Weierstrass+string) ⇒ <code>string</code>
        * [.scalarLen()](#module_curves/nist/curve..Weierstrass+scalarLen) ⇒ <code>number</code>
        * [.scalar()](#module_curves/nist/curve..Weierstrass+scalar) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.pointLen()](#module_curves/nist/curve..Weierstrass+pointLen) ⇒ <code>number</code>
        * [.point()](#module_curves/nist/curve..Weierstrass+point) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)

<a name="module_curves/nist/curve..Weierstrass"></a>

### curves/nist/curve~Weierstrass
Class Weierstrass defines the weierstrass form of
elliptic curves

**Kind**: inner class of [<code>curves/nist/curve</code>](#module_curves/nist/curve)  

* [~Weierstrass](#module_curves/nist/curve..Weierstrass)
    * [new Weierstrass(config)](#new_module_curves/nist/curve..Weierstrass_new)
    * [.string()](#module_curves/nist/curve..Weierstrass+string) ⇒ <code>string</code>
    * [.scalarLen()](#module_curves/nist/curve..Weierstrass+scalarLen) ⇒ <code>number</code>
    * [.scalar()](#module_curves/nist/curve..Weierstrass+scalar) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.pointLen()](#module_curves/nist/curve..Weierstrass+pointLen) ⇒ <code>number</code>
    * [.point()](#module_curves/nist/curve..Weierstrass+point) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)

<a name="new_module_curves/nist/curve..Weierstrass_new"></a>

#### new Weierstrass(config)
Create a new Weierstrass Curve


| Param | Type | Description |
| --- | --- | --- |
| config | <code>object</code> | Curve configuration |
| config.name | <code>String</code> | Curve name |
| config.p | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Order of the underlying field. Little Endian if string or Uint8Array. |
| config.a | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Curve Parameter a. Little Endian if string or Uint8Array. |
| config.b | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Curve Parameter b. Little Endian if string or Uint8Array. |
| config.n | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | Order of the base point. Little Endian if string or Uint8Array |
| config.gx | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | x coordinate of the base point. Little Endian if string or Uint8Array |
| config.gy | <code>String</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> | y coordinate of the base point. Little Endian if string or Uint8Array |
| config.bitSize | <code>number</code> | the size of the underlying field. |

<a name="module_curves/nist/curve..Weierstrass+string"></a>

#### weierstrass.string() ⇒ <code>string</code>
Returns the name of the curve

**Kind**: instance method of [<code>Weierstrass</code>](#module_curves/nist/curve..Weierstrass)  
<a name="module_curves/nist/curve..Weierstrass+scalarLen"></a>

#### weierstrass.scalarLen() ⇒ <code>number</code>
Returns the size in bytes of a scalar

**Kind**: instance method of [<code>Weierstrass</code>](#module_curves/nist/curve..Weierstrass)  
<a name="module_curves/nist/curve..Weierstrass+scalar"></a>

#### weierstrass.scalar() ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#module_curves/nist/curve..Weierstrass)  
<a name="module_curves/nist/curve..Weierstrass+pointLen"></a>

#### weierstrass.pointLen() ⇒ <code>number</code>
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#module_curves/nist/curve..Weierstrass)  
<a name="module_curves/nist/curve..Weierstrass+point"></a>

#### weierstrass.point() ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Returns a new Point

**Kind**: instance method of [<code>Weierstrass</code>](#module_curves/nist/curve..Weierstrass)  
<a name="module_curves/nist/point"></a>

## curves/nist/point

* [curves/nist/point](#module_curves/nist/point)
    * [~Point](#module_curves/nist/point..Point)
        * [new Point(curve, x, y)](#new_module_curves/nist/point..Point_new)
        * [.toString()](#module_curves/nist/point..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_curves/nist/point..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_curves/nist/point..Point+set) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.clone()](#module_curves/nist/point..Point+clone) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.null()](#module_curves/nist/point..Point+null) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.base()](#module_curves/nist/point..Point+base) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.embedLen()](#module_curves/nist/point..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, [callback])](#module_curves/nist/point..Point+embed) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.data()](#module_curves/nist/point..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_curves/nist/point..Point+add) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.sub(p1, p2)](#module_curves/nist/point..Point+sub) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.neg(p)](#module_curves/nist/point..Point+neg) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.mul(s, [p])](#module_curves/nist/point..Point+mul) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.pick([callback])](#module_curves/nist/point..Point+pick) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
        * [.marshalBinary()](#module_curves/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_curves/nist/point..Point+unmarshalBinary)

<a name="module_curves/nist/point..Point"></a>

### curves/nist/point~Point
Represents a Point on the nist curve

The value of the parameters is expected in little endian form if being
passed as a Uint8Array

**Kind**: inner class of [<code>curves/nist/point</code>](#module_curves/nist/point)  

* [~Point](#module_curves/nist/point..Point)
    * [new Point(curve, x, y)](#new_module_curves/nist/point..Point_new)
    * [.toString()](#module_curves/nist/point..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_curves/nist/point..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_curves/nist/point..Point+set) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.clone()](#module_curves/nist/point..Point+clone) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.null()](#module_curves/nist/point..Point+null) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.base()](#module_curves/nist/point..Point+base) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.embedLen()](#module_curves/nist/point..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, [callback])](#module_curves/nist/point..Point+embed) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.data()](#module_curves/nist/point..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_curves/nist/point..Point+add) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.sub(p1, p2)](#module_curves/nist/point..Point+sub) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.neg(p)](#module_curves/nist/point..Point+neg) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.mul(s, [p])](#module_curves/nist/point..Point+mul) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.pick([callback])](#module_curves/nist/point..Point+pick) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
    * [.marshalBinary()](#module_curves/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_curves/nist/point..Point+unmarshalBinary)

<a name="new_module_curves/nist/point..Point_new"></a>

#### new Point(curve, x, y)

| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:curves/nist/curve~Weirstrass</code> | Weierstrass curve |
| x | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |
| y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |

<a name="module_curves/nist/point..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
<a name="module_curves/nist/point..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object to compare |

<a name="module_curves/nist/point..Point+set"></a>

#### point.set(p2) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object |

<a name="module_curves/nist/point..Point+clone"></a>

#### point.clone() ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/nist/point..Point) - new Point object  
<a name="module_curves/nist/point..Point+null"></a>

#### point.null() ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Set to the neutral element for the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
<a name="module_curves/nist/point..Point+base"></a>

#### point.base() ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Set to the standard base point for this curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
<a name="module_curves/nist/point..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
<a name="module_curves/nist/point..Point+embed"></a>

#### point.embed(data, [callback]) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | data to embed with length <= embedLen |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_curves/nist/point..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_curves/nist/point..Point+add"></a>

#### point.add(p1, p2) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Returns the sum of two points on the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/nist/point..Point) - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object, addend |
| p2 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object, addend |

<a name="module_curves/nist/point..Point+sub"></a>

#### point.sub(p1, p2) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Subtract two points
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/nist/point..Point) - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object |
| p2 | [<code>Point</code>](#module_curves/nist/point..Point) | Point object |

<a name="module_curves/nist/point..Point+neg"></a>

#### point.neg(p) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Finds the negative of a point p
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Returns**: [<code>Point</code>](#module_curves/nist/point..Point) - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | [<code>Point</code>](#module_curves/nist/point..Point) | Point to negate |

<a name="module_curves/nist/point..Point+mul"></a>

#### point.mul(s, [p]) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Multiply point p by scalar s.
If p is not passed then multiplies the base point of the curve with
scalar s
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| s | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) |  | Scalar |
| [p] | [<code>Point</code>](#module_curves/nist/point..Point) | <code></code> | Point |

<a name="module_curves/nist/point..Point+pick"></a>

#### point.pick([callback]) ⇒ [<code>Point</code>](#module_curves/nist/point..Point)
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_curves/nist/point..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
converts a point into the form specified in section 4.3.6 of ANSI X9.62.

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_curves/nist/point..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes)
Convert a Uint8Array back to a curve point.
Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62

**Kind**: instance method of [<code>Point</code>](#module_curves/nist/point..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_curves/nist/scalar"></a>

## curves/nist/scalar

* [curves/nist/scalar](#module_curves/nist/scalar)
    * [~Scalar](#module_curves/nist/scalar..Scalar)
        * [new Scalar(curve, red)](#new_module_curves/nist/scalar..Scalar_new)
        * [.equal(s2)](#module_curves/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_curves/nist/scalar..Scalar+set) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.clone()](#module_curves/nist/scalar..Scalar+clone) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.zero()](#module_curves/nist/scalar..Scalar+zero) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.add(s1, s2)](#module_curves/nist/scalar..Scalar+add) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.sub(s1, s2)](#module_curves/nist/scalar..Scalar+sub) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.neg(a)](#module_curves/nist/scalar..Scalar+neg) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.one()](#module_curves/nist/scalar..Scalar+one) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.mul(s1, s2)](#module_curves/nist/scalar..Scalar+mul) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.div(s1, s2)](#module_curves/nist/scalar..Scalar+div) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.inv(a)](#module_curves/nist/scalar..Scalar+inv) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.setBytes(b)](#module_curves/nist/scalar..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.bytes()](#module_curves/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick()](#module_curves/nist/scalar..Scalar+pick) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
        * [.marshalBinary()](#module_curves/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_curves/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="module_curves/nist/scalar..Scalar"></a>

### curves/nist/scalar~Scalar
Scalar

**Kind**: inner class of [<code>curves/nist/scalar</code>](#module_curves/nist/scalar)  

* [~Scalar](#module_curves/nist/scalar..Scalar)
    * [new Scalar(curve, red)](#new_module_curves/nist/scalar..Scalar_new)
    * [.equal(s2)](#module_curves/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_curves/nist/scalar..Scalar+set) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.clone()](#module_curves/nist/scalar..Scalar+clone) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.zero()](#module_curves/nist/scalar..Scalar+zero) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.add(s1, s2)](#module_curves/nist/scalar..Scalar+add) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.sub(s1, s2)](#module_curves/nist/scalar..Scalar+sub) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.neg(a)](#module_curves/nist/scalar..Scalar+neg) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.one()](#module_curves/nist/scalar..Scalar+one) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.mul(s1, s2)](#module_curves/nist/scalar..Scalar+mul) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.div(s1, s2)](#module_curves/nist/scalar..Scalar+div) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.inv(a)](#module_curves/nist/scalar..Scalar+inv) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.setBytes(b)](#module_curves/nist/scalar..Scalar+setBytes) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.bytes()](#module_curves/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick()](#module_curves/nist/scalar..Scalar+pick) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
    * [.marshalBinary()](#module_curves/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_curves/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="new_module_curves/nist/scalar..Scalar_new"></a>

#### new Scalar(curve, red)

| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:curves/nist/curve~Weirstrass</code> |  |
| red | <code>BN.Red</code> | BN.js Reduction context |

<a name="module_curves/nist/scalar..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |

<a name="module_curves/nist/scalar..Scalar+set"></a>

#### scalar.set(a) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |

<a name="module_curves/nist/scalar..Scalar+clone"></a>

#### scalar.clone() ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+zero"></a>

#### scalar.zero() ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
**Returns**: [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |

<a name="module_curves/nist/scalar..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
**Returns**: [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |
| s2 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |

<a name="module_curves/nist/scalar..Scalar+neg"></a>

#### scalar.neg(a) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | Scalar |

<a name="module_curves/nist/scalar..Scalar+one"></a>

#### scalar.one() ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | 
| s2 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | 

<a name="module_curves/nist/scalar..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | 
| s2 | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | 

<a name="module_curves/nist/scalar..Scalar+inv"></a>

#### scalar.inv(a) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| a | [<code>Scalar</code>](#module_curves/nist/scalar..Scalar) | 

<a name="module_curves/nist/scalar..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Sets the scalar from big-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type |
| --- | --- |
| b | <code>Uint8Array</code> | 

<a name="module_curves/nist/scalar..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+pick"></a>

#### scalar.pick() ⇒ [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)
Set to a random scalar

param {function} [callback] - to generate randomBytes of given length

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (big endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  
<a name="module_curves/nist/scalar..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes) ⇒ <code>undefined</code>
Reads the binary representation (big endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_curves/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_sign/schnorr"></a>

## sign/schnorr

* [sign/schnorr](#module_sign/schnorr)
    * [~Verify(suite, publicKey, message, signature)](#module_sign/schnorr..Verify) ⇒ <code>boolean</code>
    * [~hashSchnorr(...inputs)](#module_sign/schnorr..hashSchnorr) ⇒ [<code>Scalar</code>](#Scalar)

<a name="module_sign/schnorr..Verify"></a>

### sign/schnorr~Verify(suite, publicKey, message, signature) ⇒ <code>boolean</code>
Verify verifies if the signature of the message is valid under the given public
key.

**Kind**: inner method of [<code>sign/schnorr</code>](#module_sign/schnorr)  
**Returns**: <code>boolean</code> - true if signature is valid or false otherwise.  

| Param | Type | Description |
| --- | --- | --- |
| suite | [<code>Group</code>](#Group) | suite to use |
| publicKey | [<code>Point</code>](#Point) | public key under which to verify the signature |
| message | <code>Uint8Array</code> | message that is signed |
| signature | <code>Uint8Array</code> | signature made over the given message |

<a name="module_sign/schnorr..hashSchnorr"></a>

### sign/schnorr~hashSchnorr(...inputs) ⇒ [<code>Scalar</code>](#Scalar)
hashSchnorr returns a scalar out of hashing the given inputs.

**Kind**: inner method of [<code>sign/schnorr</code>](#module_sign/schnorr)  

| Param | Type |
| --- | --- |
| ...inputs | <code>Uint8Array</code> | 

<a name="Group"></a>

## Group
Group is an abstract class for curves

**Kind**: global class  
<a name="Point"></a>

## Point
Point is an abstract class for representing
a point on an elliptic curve

**Kind**: global class  
<a name="Scalar"></a>

## Scalar
Scalar is an abstract class for representing a scalar
to be used in elliptic curve operations

**Kind**: global class  
<a name="bits"></a>

## bits(bitlen, exact, callback) ⇒ <code>Uint8Array</code>
bits choses a random Uint8Array with a maximum bitlength
If exact is `true`, chose Uint8Array with *exactly* that bitlenght not less

**Kind**: global function  

| Param | Type | Description |
| --- | --- | --- |
| bitlen | <code>number</code> | Bitlength |
| exact | <code>boolean</code> |  |
| callback | <code>function</code> | to generate random Uint8Array of given length |

<a name="int"></a>

## int(mod, callback) ⇒ <code>Uint8Array</code>
int choses a random uniform Uint8Array less than given modulus

**Kind**: global function  

| Param | Type | Description |
| --- | --- | --- |
| mod | <code>BN.jsObject</code> | modulus |
| callback | <code>function</code> | to generate a random byte array |

