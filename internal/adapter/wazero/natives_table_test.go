// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

// nativeTable classifies every native function reachable from the global
// object of the guest (SPEC-001 amendment 10, section C). TestSandboxIsolation
// BuiltinInventory fails when the guest has a native that is not here, or this
// table names one the guest lacks, so no QuickJS-ng upgrade or new built-in
// enters unclassified. In a name, "~" stands for a space ("get~Map.prototype.size").
var nativeTable = []nativeRow{
	// ---- arrays
	{class: covered, names: "Array.prototype.concat", setup: inArray, call: "a.$M(a)"},
	{class: covered, names: "Array.prototype.copyWithin", setup: inArray2, call: "a.$M(0, 1)"},
	{class: covered, names: "Array.prototype.fill", setup: inArray2, call: "a.$M(2)"},
	{class: covered, names: "Array.prototype.{every some filter find findIndex findLast findLastIndex forEach map flatMap}", setup: inArray, call: "a.$M(x => false)"},
	{class: covered, names: "Array.prototype.{reduce reduceRight}", setup: inArray, call: "a.$M((p, x) => p, 0)"},
	{class: covered, names: "Array.prototype.flat", setup: inArray, call: "a.$M()"},
	{class: covered, names: "Array.prototype.{includes indexOf lastIndexOf}", setup: inArray2, call: "a.$M(2)"},
	{class: covered, names: "Array.prototype.join", setup: inArray, call: "a.$M(',')"},
	{class: covered, names: "Array.prototype.toLocaleString", setup: inArray, call: "a.$M()"},
	{class: covered, names: "Array.prototype.{reverse toReversed}", setup: inArray, call: "a.$M()"},
	{class: covered, names: "Array.prototype.{sort toSorted}", setup: inArray, call: "a.$M()"},
	{class: covered, names: "Array.prototype.shift", setup: inArray2, call: "a.$M()", loop: "a.shift(); a.push(1)"},
	{class: covered, names: "Array.prototype.unshift", setup: inArray2, call: "a.$M(1)", loop: "a.unshift(1); a.pop()"},
	{class: covered, names: "Array.prototype.slice", setup: inArray, call: "a.$M()"},
	{class: covered, names: "Array.prototype.splice", setup: inArray2, call: "a.$M(1, 1)", loop: "a.splice(1, 0, 1); a.splice(1, 1)"},
	{class: covered, names: "Array.prototype.toSpliced", setup: inArray, call: "a.$M(1, 1)"},
	{class: covered, names: "Array.prototype.with", setup: inArray, call: "a.$M(0, 2)"},
	{class: covered, names: "Array.from", setup: "", call: "Array.from({ length: 5e6 })"},
	{class: bounded, names: "Array Array.of Array.isArray Array.prototype.{at pop push entries keys values}", why: "O(1), or bounded by the call arguments, which the stack caps"},
	{class: bounded, names: "get~Array.[Symbol.species]", why: "returns a constant"},

	// ---- objects
	{class: covered, names: "Object.assign", setup: inObj, call: "Object.$M({}, o)"},
	{class: covered, names: "Object.defineProperties", setup: inObj, call: "Object.$M({}, Object.getOwnPropertyDescriptors(o))"},
	{class: covered, names: "Object.{entries keys values getOwnPropertyNames getOwnPropertySymbols getOwnPropertyDescriptors isFrozen isSealed}", setup: inObj, call: "Object.$M(o)"},
	{class: covered, names: "Object.{freeze seal}", setup: inObj, call: "Object.$M(Object.assign({}, o))"},
	{class: covered, names: "Object.fromEntries", setup: inObj + "const en = Object.entries(o);", call: "Object.$M(en)"},
	{class: covered, names: "Object.groupBy Map.groupBy", setup: inArray, call: "$O.$M(a, x => 1)"},
	{class: bounded, names: "Object Object.{create defineProperty getOwnPropertyDescriptor getPrototypeOf hasOwn is isExtensible preventExtensions setPrototypeOf}", why: "one property or one object"},
	{class: bounded, names: ".__proto__.{__defineGetter__ __defineSetter__ __lookupGetter__ __lookupSetter__ hasOwnProperty isPrototypeOf propertyIsEnumerable toLocaleString toString valueOf} get~.__proto__.__proto__ set~.__proto__.__proto__", why: "one property"},
	{class: bounded, names: "Proxy Proxy.revocable Reflect.{apply construct defineProperty deleteProperty get getOwnPropertyDescriptor getPrototypeOf has isExtensible preventExtensions set setPrototypeOf}", why: "one property or one call"},
	{class: covered, names: "Reflect.ownKeys", setup: inObj, call: "Reflect.$M(o)"},

	// ---- functions, evaluation and errors
	{class: covered, names: "eval", setup: "const src = '1+'.repeat(2e6) + '1';", call: "eval(src)"},
	{class: covered, names: "Function", setup: "const src = '1+'.repeat(2e6) + '1';", call: "Function('return ' + src)"},
	{class: bounded, names: "Function.prototype %GeneratorFunction%.constructor %GeneratorFunction%.__proto__.{[Symbol.hasInstance] apply bind call toString} get~%GeneratorFunction%.__proto__.{caller columnNumber fileName lineNumber}", why: "one call; the arguments of apply and bind are capped by the stack"},
	{class: bounded, names: "Error Error.{captureStackTrace isError} Error.prototype.toString get~Error.{prepareStackTrace stackTraceLimit} set~Error.{prepareStackTrace stackTraceLimit} get~Error.prototype.stack set~Error.prototype.stack {AggregateError EvalError InternalError RangeError ReferenceError SuppressedError SyntaxError TypeError URIError DOMException} get~DOMException.prototype.{code message name}", why: "the stack trace is capped by stackTraceLimit"},
	{class: bounded, names: "queueMicrotask", why: "queues one job"},

	// ---- generators and iterators
	{class: bounded, names: "%IteratorHelper%.__proto__.return %GeneratorFunction%.prototype.{next return throw} %ArrayIterator%.__proto__.next %MapIterator%.__proto__.next %SetIterator%.__proto__.next %StringIterator%.__proto__.next %RegExpStringIterator%.__proto__.next %WrapForValidIterator%.__proto__.{next return}", why: "one step; a generator body is JavaScript and checks the deadline by bytecode"},
	{class: covered, names: "%IteratorHelper%.__proto__.next", setup: inArray, call: "a.values().drop(1e9).next()"},
	{class: covered, names: "Iterator.prototype.{toArray join}", setup: inArray, call: "a.values().$M()"},
	{class: covered, names: "Iterator.prototype.{forEach every some find includes}", setup: inArray, call: "a.values().$M(x => false)"},
	{class: covered, names: "Iterator.prototype.reduce", setup: inArray, call: "a.values().$M((p, x) => p, 0)"},
	{class: covered, names: "Iterator.prototype.{map filter flatMap}", setup: inArray, call: "a.values().$M(x => [x]).toArray()"},
	{class: covered, names: "Iterator.prototype.{drop take}", setup: inArray, call: "a.values().$M(1e9).toArray()"},
	{class: covered, names: "Iterator.prototype.{chunks windows}", setup: inArray, call: "a.values().$M(2).toArray()"},
	{class: covered, names: "Iterator.concat", setup: inArray, call: "Iterator.concat(a.values()).toArray()"},
	{class: covered, names: "Iterator.zip", setup: inArray, call: "Iterator.zip([a.values(), a.values()]).toArray()"},
	{class: covered, names: "Iterator.zipKeyed", setup: inArray, call: "Iterator.zipKeyed({ x: a.values() }).toArray()"},
	{class: bounded, names: "Iterator Iterator.from Iterator.prototype.{[Symbol.dispose] [Symbol.iterator]} get~Iterator.prototype.{[Symbol.toStringTag] constructor} set~Iterator.prototype.{[Symbol.toStringTag] constructor}", why: "one call"},

	// ---- strings
	{class: covered, names: "String.prototype.concat", setup: inStr, call: "s.$M(s)"},
	{class: covered, names: "String.prototype.{endsWith startsWith}", setup: inStr, call: "s.$M('a'.repeat(5e6))"},
	{class: covered, names: "String.prototype.{includes indexOf lastIndexOf}", setup: inStr, call: "s.$M('a'.repeat(2000) + 'b')"},
	{class: covered, names: "String.prototype.localeCompare", setup: inStrM, call: "s.$M(s)"},
	{class: covered, names: "String.prototype.{match search}", setup: inStr, call: "s.$M(/b/)"},
	{class: covered, names: "String.prototype.matchAll", setup: inStr, call: "s.$M(/a/g).toArray()"},
	{class: covered, names: "String.prototype.normalize", setup: inWide, call: "w.$M('NFD')"},
	{class: covered, names: "String.prototype.{padEnd padStart}", setup: inStr, call: "''.$M(2e7, 'ab')"},
	{class: covered, names: "String.prototype.repeat", setup: inStr, call: "s.$M(3)"},
	{class: covered, names: "String.prototype.{replace replaceAll}", setup: inStr, call: "s.$M('a', 'bb')"},
	{class: covered, names: "String.prototype.slice", setup: inStr, call: "s.$M(1)"},
	{class: covered, names: "String.prototype.split", setup: inStrM, call: "s.$M('')"},
	{class: covered, names: "String.prototype.{substr substring}", setup: inStr, call: "s.$M(1)"},
	{class: covered, names: "String.prototype.{toLowerCase toUpperCase toLocaleLowerCase toLocaleUpperCase}", setup: inWide, call: "w.$M()"},
	{class: covered, names: "String.prototype.{toWellFormed isWellFormed}", setup: inStr, call: "s.$M()"},
	{class: covered, names: "String.prototype.{trim trimEnd trimStart}", setup: inSpaces, call: "s.$M()"},
	{class: covered, names: "String.prototype.{big blink bold fixed italics small strike sub sup}", setup: inStr, call: "s.$M()"},
	{class: covered, names: "String.prototype.{anchor fontcolor fontsize link}", setup: inStr, call: "s.$M('x')"},
	{class: covered, names: "String.raw", setup: inStr, call: "String.raw({ raw: s }, 1)"},
	{class: covered, names: "String.prototype.[Symbol.iterator]", setup: inStrM, call: "[...s]"},
	{class: bounded, names: "String String.{fromCharCode fromCodePoint} String.prototype.{at charAt charCodeAt codePointAt toString valueOf}", why: "one character, or bounded by the call arguments, which the stack caps"},

	// ---- regular expressions
	{class: covered, names: "RegExp.prototype.{exec test}", setup: inStrM, call: "/a*b/.$M(s)"},
	{class: covered, names: "RegExp.prototype.[Symbol.match]", setup: inStr, call: "/a/g[Symbol.match](s)"},
	{class: covered, names: "RegExp.prototype.[Symbol.matchAll]", setup: inStr, call: "/a/g[Symbol.matchAll](s).toArray()"},
	{class: covered, names: "RegExp.prototype.[Symbol.replace]", setup: inStr, call: "/a/g[Symbol.replace](s, 'bb')"},
	{class: covered, names: "RegExp.prototype.[Symbol.search]", setup: inStr, call: "/b/[Symbol.search](s)"},
	{class: covered, names: "RegExp.prototype.[Symbol.split]", setup: inStr, call: "/a/[Symbol.split](s)"},
	{class: covered, names: "RegExp.escape", setup: inStr, call: "RegExp.$M(s)"},
	{class: covered, names: "RegExp.prototype.compile", setup: inStr, call: "/x/.compile('a'.repeat(1e7))"},
	{class: covered, names: "RegExp", setup: "const s = 'a'.repeat(3e6);", call: "new RegExp(s)"},
	{class: covered, names: "get~RegExp.prototype.source", setup: inStr, call: "new RegExp(s.slice(0, 1e6)).source"},
	{class: bounded, names: "RegExp.prototype.toString get~RegExp.[Symbol.species] get~RegExp.prototype.{dotAll flags global hasIndices ignoreCase multiline sticky unicode unicodeSets}", why: "the source is read once, or a flag is read"},

	// ---- JSON, global functions and encodings
	{class: covered, names: "JSON.parse", setup: inJSON, call: "JSON.$M(j)"},
	{class: covered, names: "JSON.stringify", setup: inArray, call: "JSON.$M(a)"},
	{class: bounded, names: "JSON.{isRawJSON rawJSON}", why: "one value"},
	{class: covered, names: "atob", setup: "const s = 'QUJD'.repeat(4e6);", call: "atob(s)"},
	{class: covered, names: "btoa", setup: inStr, call: "btoa(s)"},
	{class: covered, names: "escape", setup: inSpaces, call: "escape(s)"},
	{class: covered, names: "unescape", setup: "const s = '%20'.repeat(5e6);", call: "unescape(s)"},
	{class: covered, names: "encodeURI encodeURIComponent", setup: inURI, call: "$M(e)"},
	{class: covered, names: "decodeURI decodeURIComponent", setup: "const e = '%C3%A9'.repeat(3e6);", call: "$M(e)"},
	{class: bounded, names: "isFinite isNaN parseFloat parseInt", why: "numeric parsing of a string stops at the first character that is not part of a number; its length is capped by the memory"},

	// ---- maps and sets
	{class: covered, names: "Map", setup: inArray, call: "new Map(a.entries())"},
	{class: covered, names: "Set", setup: inArray, call: "new Set(a)"},
	{class: covered, names: "Set.prototype.clear", setup: inSet, call: "new Set(st).clear()"},
	{class: covered, names: "Map.prototype.clear", setup: inMap, call: "new Map(m).clear()"},
	{class: covered, names: "Map.prototype.forEach", setup: inMap, call: "m.$M(x => 0)"},
	{class: covered, names: "Set.prototype.forEach", setup: inSet, call: "st.$M(x => 0)"},
	{class: covered, names: "Set.prototype.{union intersection difference symmetricDifference isSubsetOf isSupersetOf isDisjointFrom}", setup: inSet, call: "st.$M(new Set(st))"},
	{class: bounded, names: "Map.prototype.{delete get getOrInsert getOrInsertComputed has set entries keys values} Set.prototype.{add delete has entries values} get~Map.[Symbol.species] get~Set.[Symbol.species] get~Map.prototype.size get~Set.prototype.size WeakMap.prototype.{delete get getOrInsert getOrInsertComputed has set} WeakSet.prototype.{add delete has} WeakMap WeakSet", why: "one hash lookup; growing a table once rehashes it in a few milliseconds"},

	// ---- numbers, bigints, symbols, booleans, math, dates
	{class: bounded, names: "Number Number.{isFinite isInteger isNaN isSafeInteger} Number.prototype.{toExponential toFixed toPrecision toString valueOf} Boolean Boolean.prototype.{toString valueOf} Symbol Symbol.{for keyFor} Symbol.prototype.{[Symbol.toPrimitive] toString valueOf} get~Symbol.prototype.description", why: "one number or symbol"},
	{class: covered, names: "BigInt", setup: "const s = '9'.repeat(300000);", call: "BigInt(s)"},
	{class: covered, names: "BigInt.prototype.toString", setup: "const n = (1n << 1000000n) - 1n;", call: "n.toString()"},
	{class: covered, names: "BigInt.{asIntN asUintN}", setup: "", call: "BigInt.$M(1e6, -1n)"},
	{class: bounded, names: "BigInt.prototype.valueOf", why: "returns the value"},
	{class: bounded, names: "Math.{abs acos acosh asin asinh atan atan2 atanh cbrt ceil clz32 cos cosh exp expm1 f16round floor fround imul log log10 log1p log2 pow round sign sin sinh sqrt tan tanh trunc}", why: "one number"},
	{class: bounded, names: "Math.{hypot max min sumPrecise}", why: "bounded by the call arguments, which the stack caps"},
	{class: bounded, names: "Date Date.{UTC now parse} Date.prototype.{[Symbol.toPrimitive] getDate getDay getFullYear getHours getMilliseconds getMinutes getMonth getSeconds getTime getTimezoneOffset getUTCDate getUTCDay getUTCFullYear getUTCHours getUTCMilliseconds getUTCMinutes getUTCMonth getUTCSeconds getYear setDate setFullYear setHours setMilliseconds setMinutes setMonth setSeconds setTime setUTCDate setUTCFullYear setUTCHours setUTCMilliseconds setUTCMinutes setUTCMonth setUTCSeconds setYear toDateString toISOString toJSON toString toTimeString toUTCString valueOf}", why: "one date; Date.parse reads at most a short prefix"},

	// ---- array buffers, typed arrays and data views
	{class: covered, names: "ArrayBuffer SharedArrayBuffer", setup: "", call: "new $M(3e7)"},
	{class: covered, names: "ArrayBuffer.prototype.{slice sliceToImmutable}", setup: inBuf, call: "b.$M(1)"},
	{class: covered, names: "ArrayBuffer.prototype.{transfer transferToFixedLength}", setup: "let b = new ArrayBuffer(2e7);", call: "b = b.$M()"},
	{class: covered, names: "ArrayBuffer.prototype.transferToImmutable", setup: "let b = new ArrayBuffer(2e7);", call: "b.$M()", loop: "new ArrayBuffer(2e7).$M()"},
	{class: covered, names: "ArrayBuffer.prototype.resize", setup: "const b = new ArrayBuffer(1, { maxByteLength: 3e7 });", call: "b.$M(3e7); b.resize(1)"},
	{class: covered, names: "SharedArrayBuffer.prototype.slice", setup: "const b = new SharedArrayBuffer(2e7);", call: "b.$M(1)"},
	{class: bounded, names: "ArrayBuffer.isView DataView DataView.prototype.{getBigInt64 getBigUint64 getFloat16 getFloat32 getFloat64 getInt16 getInt32 getInt8 getUint16 getUint32 getUint8 setBigInt64 setBigUint64 setFloat16 setFloat32 setFloat64 setInt16 setInt32 setInt8 setUint16 setUint32 setUint8} get~ArrayBuffer.[Symbol.species] get~ArrayBuffer.prototype.{byteLength detached immutable maxByteLength resizable} get~SharedArrayBuffer.[Symbol.species] get~SharedArrayBuffer.prototype.{byteLength growable maxByteLength} SharedArrayBuffer.prototype.grow get~DataView.prototype.{buffer byteLength byteOffset}", why: "one element, or a view that copies nothing"},
	{class: covered, names: "{Int8Array Uint8Array Uint8ClampedArray Int16Array Uint16Array Int32Array Uint32Array Float16Array Float32Array Float64Array}", setup: inArray, call: "new $M(a)"},
	{class: covered, names: "{BigInt64Array BigUint64Array}", setup: "", call: "new $M(3e6)"},
	{class: covered, names: "%TypedArray%.{from of}", setup: inArray, call: "Float64Array.$M(a)"},
	{class: covered, names: "%TypedArray%.prototype.{copyWithin}", setup: inF64, call: "t.$M(0, 1)"},
	{class: covered, names: "%TypedArray%.prototype.fill", setup: inF64, call: "t.$M(2)"},
	{class: covered, names: "%TypedArray%.prototype.{every some filter find findIndex findLast findLastIndex forEach map}", setup: inF64, call: "t.$M(x => false)"},
	{class: covered, names: "%TypedArray%.prototype.{reduce reduceRight}", setup: inF64, call: "t.$M((p, x) => p, 0)"},
	{class: covered, names: "%TypedArray%.prototype.{includes indexOf lastIndexOf}", setup: inF64, call: "t.$M(2)"},
	{class: covered, names: "%TypedArray%.prototype.{join toLocaleString toString}", setup: inF64, call: "t.$M()"},
	{class: covered, names: "%TypedArray%.prototype.{reverse toReversed}", setup: inF64, call: "t.$M()"},
	{class: covered, names: "%TypedArray%.prototype.{sort toSorted}", setup: inF64, call: "t.$M()"},
	{class: covered, names: "%TypedArray%.prototype.set", setup: inF64, call: "t.$M(t)"},
	{class: covered, names: "%TypedArray%.prototype.slice", setup: inF64, call: "t.$M()"},
	{class: covered, names: "%TypedArray%.prototype.with", setup: inF64, call: "t.$M(0, 2)"},
	{class: covered, names: "Uint8Array.prototype.toBase64", setup: inU8, call: "u.$M()"},
	{class: covered, names: "Uint8Array.prototype.toHex", setup: "const u = new Uint8Array(1e7);", call: "u.$M()"},
	{class: covered, names: "Uint8Array.fromBase64", setup: "const s = 'QUJD'.repeat(4e6);", call: "Uint8Array.$M(s)"},
	{class: covered, names: "Uint8Array.fromHex", setup: "const s = 'ab'.repeat(8e6);", call: "Uint8Array.$M(s)"},
	{class: covered, names: "Uint8Array.prototype.setFromBase64", setup: "const s = 'QUJD'.repeat(4e6); const u = new Uint8Array(2e7);", call: "u.$M(s)"},
	{class: covered, names: "Uint8Array.prototype.setFromHex", setup: "const s = 'ab'.repeat(8e6); const u = new Uint8Array(2e7);", call: "u.$M(s)"},
	{class: bounded, names: "%TypedArray%.prototype.{at constructor entries keys values subarray} get~%TypedArray%.[Symbol.species] get~%TypedArray%.prototype.{[Symbol.toStringTag] buffer byteLength byteOffset length}", why: "one element, or a view that copies nothing"},

	// ---- resource management and the rest
	{class: covered, names: "DisposableStack.prototype.dispose", setup: "const d = new DisposableStack(); for (let i = 0; i < 2e5; i++) d.defer(() => 0);", call: "d.$M()"},
	{class: bounded, names: "DisposableStack DisposableStack.prototype.{adopt defer move use} get~DisposableStack.prototype.disposed", why: "registers one resource"},
}
