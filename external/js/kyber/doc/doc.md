## Modules

<dl>
<dt><a href="#module_constants">constants</a></dt>
<dd></dd>
<dt><a href="#module_group/nist/point">group/nist/point</a></dt>
<dd></dd>
<dt><a href="#module_group/nist/scalar">group/nist/scalar</a></dt>
<dd></dd>
</dl>

## Classes

<dl>
<dt><a href="#Weierstrass">Weierstrass</a></dt>
<dd><p>Class Weierstrass defines the weierstrass form of
elliptic curves</p>
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

<a name="module_group/nist/point"></a>

## group/nist/point

* [group/nist/point](#module_group/nist/point)
    * [~Point](#module_group/nist/point..Point)
        * [new Point(curve, x, y)](#new_module_group/nist/point..Point_new)
        * [.toString()](#module_group/nist/point..Point+toString) ⇒ <code>string</code>
        * [.equal(p2)](#module_group/nist/point..Point+equal) ⇒ <code>boolean</code>
        * [.set(p2)](#module_group/nist/point..Point+set) ⇒ <code>module:group/nist~Point</code>
        * [.clone()](#module_group/nist/point..Point+clone) ⇒ <code>module:group/nist~Point</code>
        * [.null()](#module_group/nist/point..Point+null) ⇒ <code>module:group/nist~Point</code>
        * [.base()](#module_group/nist/point..Point+base) ⇒ <code>module:group/nist~Point</code>
        * [.embedLen()](#module_group/nist/point..Point+embedLen) ⇒ <code>number</code>
        * [.embed(data, [callback])](#module_group/nist/point..Point+embed) ⇒ <code>module:group/nist~Point</code>
        * [.data()](#module_group/nist/point..Point+data) ⇒ <code>Uint8Array</code>
        * [.add(p1, p2)](#module_group/nist/point..Point+add) ⇒ <code>module:group/nist~Point</code>
        * [.sub(p1, p2)](#module_group/nist/point..Point+sub) ⇒ <code>module:group/nist~Point</code>
        * [.neg(p)](#module_group/nist/point..Point+neg) ⇒ <code>module:group/nist~Point</code>
        * [.mul(s, [p])](#module_group/nist/point..Point+mul) ⇒ <code>module:group/nist~Point</code>
        * [.pick([callback])](#module_group/nist/point..Point+pick) ⇒ <code>module:group/nist~Point</code>
        * [.marshalBinary()](#module_group/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist/point..Point+unmarshalBinary)

<a name="module_group/nist/point..Point"></a>

### group/nist/point~Point
**Kind**: inner class of [<code>group/nist/point</code>](#module_group/nist/point)  

* [~Point](#module_group/nist/point..Point)
    * [new Point(curve, x, y)](#new_module_group/nist/point..Point_new)
    * [.toString()](#module_group/nist/point..Point+toString) ⇒ <code>string</code>
    * [.equal(p2)](#module_group/nist/point..Point+equal) ⇒ <code>boolean</code>
    * [.set(p2)](#module_group/nist/point..Point+set) ⇒ <code>module:group/nist~Point</code>
    * [.clone()](#module_group/nist/point..Point+clone) ⇒ <code>module:group/nist~Point</code>
    * [.null()](#module_group/nist/point..Point+null) ⇒ <code>module:group/nist~Point</code>
    * [.base()](#module_group/nist/point..Point+base) ⇒ <code>module:group/nist~Point</code>
    * [.embedLen()](#module_group/nist/point..Point+embedLen) ⇒ <code>number</code>
    * [.embed(data, [callback])](#module_group/nist/point..Point+embed) ⇒ <code>module:group/nist~Point</code>
    * [.data()](#module_group/nist/point..Point+data) ⇒ <code>Uint8Array</code>
    * [.add(p1, p2)](#module_group/nist/point..Point+add) ⇒ <code>module:group/nist~Point</code>
    * [.sub(p1, p2)](#module_group/nist/point..Point+sub) ⇒ <code>module:group/nist~Point</code>
    * [.neg(p)](#module_group/nist/point..Point+neg) ⇒ <code>module:group/nist~Point</code>
    * [.mul(s, [p])](#module_group/nist/point..Point+mul) ⇒ <code>module:group/nist~Point</code>
    * [.pick([callback])](#module_group/nist/point..Point+pick) ⇒ <code>module:group/nist~Point</code>
    * [.marshalBinary()](#module_group/nist/point..Point+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist/point..Point+unmarshalBinary)

<a name="new_module_group/nist/point..Point_new"></a>

#### new Point(curve, x, y)
Represents a Point on the nist curve

The value of the parameters is expected in little endian form if being
passed as a Uint8Array


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> | Weierstrass curve |
| x | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |
| y | <code>number</code> \| <code>Uint8Array</code> \| <code>BN.jsObject</code> |  |

<a name="module_group/nist/point..Point+toString"></a>

#### point.toString() ⇒ <code>string</code>
Returns the little endian representation of the y coordinate of
the Point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+equal"></a>

#### point.equal(p2) ⇒ <code>boolean</code>
Tests for equality between two Points derived from the same group

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/nist~Point</code> | Point object to compare |

<a name="module_group/nist/point..Point+set"></a>

#### point.set(p2) ⇒ <code>module:group/nist~Point</code>
set Set the current point to be equal to p2

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| p2 | <code>module:group/nist~Point</code> | Point object |

<a name="module_group/nist/point..Point+clone"></a>

#### point.clone() ⇒ <code>module:group/nist~Point</code>
Creates a copy of the current point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - new Point object  
<a name="module_group/nist/point..Point+null"></a>

#### point.null() ⇒ <code>module:group/nist~Point</code>
Set to the neutral element for the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+base"></a>

#### point.base() ⇒ <code>module:group/nist~Point</code>
Set to the standard base point for this curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+embedLen"></a>

#### point.embedLen() ⇒ <code>number</code>
Returns the length (in bytes) of the embedded data

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
<a name="module_group/nist/point..Point+embed"></a>

#### point.embed(data, [callback]) ⇒ <code>module:group/nist~Point</code>
Returns a Point with data embedded in the y coordinate

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>TypeError</code> if data is not Uint8Array
- <code>Error</code> if data.length > embedLen


| Param | Type | Description |
| --- | --- | --- |
| data | <code>Uint8Array</code> | data to embed with length <= embedLen |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist/point..Point+data"></a>

#### point.data() ⇒ <code>Uint8Array</code>
Extract embedded data from a point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>Error</code> when length of embedded data > embedLen

<a name="module_group/nist/point..Point+add"></a>

#### point.add(p1, p2) ⇒ <code>module:group/nist~Point</code>
Returns the sum of two points on the curve
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - p1 + p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/nist~Point</code> | Point object, addend |
| p2 | <code>module:group/nist~Point</code> | Point object, addend |

<a name="module_group/nist/point..Point+sub"></a>

#### point.sub(p1, p2) ⇒ <code>module:group/nist~Point</code>
Subtract two points
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - p1 - p2  

| Param | Type | Description |
| --- | --- | --- |
| p1 | <code>module:group/nist~Point</code> | Point object |
| p2 | <code>module:group/nist~Point</code> | Point object |

<a name="module_group/nist/point..Point+neg"></a>

#### point.neg(p) ⇒ <code>module:group/nist~Point</code>
Finds the negative of a point p
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>module:group/nist~Point</code> - -p  

| Param | Type | Description |
| --- | --- | --- |
| p | <code>module:group/nist~Point</code> | Point to negate |

<a name="module_group/nist/point..Point+mul"></a>

#### point.mul(s, [p]) ⇒ <code>module:group/nist~Point</code>
Multiply point p by scalar s.
If p is not passed then multiplies the base point of the curve with
scalar s
Modifies the receiver

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| s | <code>module:group/nist~Scalar</code> |  | Scalar |
| [p] | <code>module:group/nist~Point</code> | <code></code> | Point |

<a name="module_group/nist/point..Point+pick"></a>

#### point.pick([callback]) ⇒ <code>module:group/nist~Point</code>
Selects a random point

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  

| Param | Type | Description |
| --- | --- | --- |
| [callback] | <code>function</code> | to generate a random byte array of given length |

<a name="module_group/nist/point..Point+marshalBinary"></a>

#### point.marshalBinary() ⇒ <code>Uint8Array</code>
converts a point into the form specified in section 4.3.6 of ANSI X9.62.

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Returns**: <code>Uint8Array</code> - byte representation  
<a name="module_group/nist/point..Point+unmarshalBinary"></a>

#### point.unmarshalBinary(bytes)
Convert a Uint8Array back to a curve point.
Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62

**Kind**: instance method of [<code>Point</code>](#module_group/nist/point..Point)  
**Throws**:

- <code>TypeError</code> when bytes is not Uint8Array
- <code>Error</code> when bytes does not correspond to a valid point


| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="module_group/nist/scalar"></a>

## group/nist/scalar

* [group/nist/scalar](#module_group/nist/scalar)
    * [~Scalar](#module_group/nist/scalar..Scalar)
        * [new Scalar(curve, red)](#new_module_group/nist/scalar..Scalar_new)
        * [.equal(s2)](#module_group/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
        * [.set(a)](#module_group/nist/scalar..Scalar+set) ⇒ <code>module:group/nist~Scalar</code>
        * [.clone()](#module_group/nist/scalar..Scalar+clone) ⇒ <code>module:group/nist~Scalar</code>
        * [.zero()](#module_group/nist/scalar..Scalar+zero) ⇒ <code>module:group/nist~Scalar</code>
        * [.add(s1, s2)](#module_group/nist/scalar..Scalar+add) ⇒ <code>module:group/nist~Scalar</code>
        * [.sub(s1, s2)](#module_group/nist/scalar..Scalar+sub) ⇒ <code>module:group/nist~Scalar</code>
        * [.neg(a)](#module_group/nist/scalar..Scalar+neg) ⇒ <code>module:group/nist~Scalar</code>
        * [.one()](#module_group/nist/scalar..Scalar+one) ⇒ <code>module:group/nist~Scalar</code>
        * [.mul(s1, s2)](#module_group/nist/scalar..Scalar+mul) ⇒ <code>module:group/nist~Scalar</code>
        * [.div(s1, s2)](#module_group/nist/scalar..Scalar+div) ⇒ <code>module:group/nist~Scalar</code>
        * [.inv(a)](#module_group/nist/scalar..Scalar+inv) ⇒ <code>module:group/nist~Scalar</code>
        * [.setBytes(b)](#module_group/nist/scalar..Scalar+setBytes) ⇒ <code>module:group/nist~Scalar</code>
        * [.bytes()](#module_group/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
        * [.pick()](#module_group/nist/scalar..Scalar+pick) ⇒ <code>module:group/nist~Scalar</code>
        * [.marshalBinary()](#module_group/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
        * [.unmarshalBinary(bytes)](#module_group/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="module_group/nist/scalar..Scalar"></a>

### group/nist/scalar~Scalar
**Kind**: inner class of [<code>group/nist/scalar</code>](#module_group/nist/scalar)  

* [~Scalar](#module_group/nist/scalar..Scalar)
    * [new Scalar(curve, red)](#new_module_group/nist/scalar..Scalar_new)
    * [.equal(s2)](#module_group/nist/scalar..Scalar+equal) ⇒ <code>boolean</code>
    * [.set(a)](#module_group/nist/scalar..Scalar+set) ⇒ <code>module:group/nist~Scalar</code>
    * [.clone()](#module_group/nist/scalar..Scalar+clone) ⇒ <code>module:group/nist~Scalar</code>
    * [.zero()](#module_group/nist/scalar..Scalar+zero) ⇒ <code>module:group/nist~Scalar</code>
    * [.add(s1, s2)](#module_group/nist/scalar..Scalar+add) ⇒ <code>module:group/nist~Scalar</code>
    * [.sub(s1, s2)](#module_group/nist/scalar..Scalar+sub) ⇒ <code>module:group/nist~Scalar</code>
    * [.neg(a)](#module_group/nist/scalar..Scalar+neg) ⇒ <code>module:group/nist~Scalar</code>
    * [.one()](#module_group/nist/scalar..Scalar+one) ⇒ <code>module:group/nist~Scalar</code>
    * [.mul(s1, s2)](#module_group/nist/scalar..Scalar+mul) ⇒ <code>module:group/nist~Scalar</code>
    * [.div(s1, s2)](#module_group/nist/scalar..Scalar+div) ⇒ <code>module:group/nist~Scalar</code>
    * [.inv(a)](#module_group/nist/scalar..Scalar+inv) ⇒ <code>module:group/nist~Scalar</code>
    * [.setBytes(b)](#module_group/nist/scalar..Scalar+setBytes) ⇒ <code>module:group/nist~Scalar</code>
    * [.bytes()](#module_group/nist/scalar..Scalar+bytes) ⇒ <code>Uint8Array</code>
    * [.pick()](#module_group/nist/scalar..Scalar+pick) ⇒ <code>module:group/nist~Scalar</code>
    * [.marshalBinary()](#module_group/nist/scalar..Scalar+marshalBinary) ⇒ <code>Uint8Array</code>
    * [.unmarshalBinary(bytes)](#module_group/nist/scalar..Scalar+unmarshalBinary) ⇒ <code>undefined</code>

<a name="new_module_group/nist/scalar..Scalar_new"></a>

#### new Scalar(curve, red)
Scalar


| Param | Type | Description |
| --- | --- | --- |
| curve | <code>module:group/nist~Weierstrass</code> |  |
| red | <code>BN.Red</code> | BN.js Reduction context |

<a name="module_group/nist/scalar..Scalar+equal"></a>

#### scalar.equal(s2) ⇒ <code>boolean</code>
Equality test for two Scalars derived from the same Group

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+set"></a>

#### scalar.set(a) ⇒ <code>module:group/nist~Scalar</code>
Sets the receiver equal to another Scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+clone"></a>

#### scalar.clone() ⇒ <code>module:group/nist~Scalar</code>
Returns a copy of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+zero"></a>

#### scalar.zero() ⇒ <code>module:group/nist~Scalar</code>
Set to the additive identity (0)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+add"></a>

#### scalar.add(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular sums of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Returns**: <code>module:group/nist~Scalar</code> - s1 + s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | Scalar |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+sub"></a>

#### scalar.sub(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular difference

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Returns**: <code>module:group/nist~Scalar</code> - s1 - s2  

| Param | Type | Description |
| --- | --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | Scalar |
| s2 | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+neg"></a>

#### scalar.neg(a) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular negation of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type | Description |
| --- | --- | --- |
| a | <code>module:group/nist~Scalar</code> | Scalar |

<a name="module_group/nist/scalar..Scalar+one"></a>

#### scalar.one() ⇒ <code>module:group/nist~Scalar</code>
Set to the multiplicative identity (1)

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+mul"></a>

#### scalar.mul(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular products of scalars s1 and s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | 
| s2 | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+div"></a>

#### scalar.div(s1, s2) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular division of scalar s1 by scalar s2

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| s1 | <code>module:group/nist~Scalar</code> | 
| s2 | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+inv"></a>

#### scalar.inv(a) ⇒ <code>module:group/nist~Scalar</code>
Set to the modular inverse of scalar a

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| a | <code>module:group/nist~Scalar</code> | 

<a name="module_group/nist/scalar..Scalar+setBytes"></a>

#### scalar.setBytes(b) ⇒ <code>module:group/nist~Scalar</code>
Sets the scalar from big-endian Uint8Array
and reduces to the appropriate modulus

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
**Throws**:

- <code>TypeError</code> when b is not Uint8Array


| Param | Type |
| --- | --- |
| b | <code>Uint8Array</code> | 

<a name="module_group/nist/scalar..Scalar+bytes"></a>

#### scalar.bytes() ⇒ <code>Uint8Array</code>
Returns a big-endian representation of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+pick"></a>

#### scalar.pick() ⇒ <code>module:group/nist~Scalar</code>
Set to a random scalar

param {function} [callback] - to generate randomBytes of given length

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+marshalBinary"></a>

#### scalar.marshalBinary() ⇒ <code>Uint8Array</code>
Returns the binary representation (big endian) of the scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  
<a name="module_group/nist/scalar..Scalar+unmarshalBinary"></a>

#### scalar.unmarshalBinary(bytes) ⇒ <code>undefined</code>
Reads the binary representation (big endian) of scalar

**Kind**: instance method of [<code>Scalar</code>](#module_group/nist/scalar..Scalar)  

| Param | Type |
| --- | --- |
| bytes | <code>Uint8Array</code> | 

<a name="Weierstrass"></a>

## Weierstrass
Class Weierstrass defines the weierstrass form of
elliptic curves

**Kind**: global class  

* [Weierstrass](#Weierstrass)
    * [new Weierstrass(config)](#new_Weierstrass_new)
    * [.string()](#Weierstrass+string) ⇒ <code>string</code>
    * [.scalarLen()](#Weierstrass+scalarLen) ⇒ <code>number</code>
    * [.scalar()](#Weierstrass+scalar) ⇒ [<code>Scalar</code>](#module_group/nist/scalar..Scalar)
    * [.pointLen()](#Weierstrass+pointLen) ⇒ <code>number</code>
    * [.point()](#Weierstrass+point) ⇒ [<code>Point</code>](#module_group/nist/point..Point)

<a name="new_Weierstrass_new"></a>

### new Weierstrass(config)
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

<a name="Weierstrass+string"></a>

### weierstrass.string() ⇒ <code>string</code>
Returns the name of the curve

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+scalarLen"></a>

### weierstrass.scalarLen() ⇒ <code>number</code>
Returns the size in bytes of a scalar

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+scalar"></a>

### weierstrass.scalar() ⇒ [<code>Scalar</code>](#module_group/nist/scalar..Scalar)
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+pointLen"></a>

### weierstrass.pointLen() ⇒ <code>number</code>
Returns the size in bytes of a point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
<a name="Weierstrass+point"></a>

### weierstrass.point() ⇒ [<code>Point</code>](#module_group/nist/point..Point)
Returns a new Point

**Kind**: instance method of [<code>Weierstrass</code>](#Weierstrass)  
