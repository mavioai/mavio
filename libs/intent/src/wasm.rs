//! The WASM module's exports. A caller allocates a buffer with
//! `intent_alloc`, writes a request's binary message into it, calls a
//! function and frees the buffer with `intent_free`; a function returns its
//! answer's message as a buffer it allocated, its address in the high 32 bits
//! and its length in the low ones, which the caller frees too. 0 means the
//! request could not be read.

use buffa::Message;

use crate::proto::{ExamplesRequest, ParseIntentRequest, ReplyRequest};

fn give(bytes: Vec<u8>) -> u64 {
    let len = bytes.len();
    let ptr = Box::into_raw(bytes.into_boxed_slice()) as *mut u8;
    ((ptr as u64) << 32) | len as u64
}

/// # Safety
/// The buffer must be one `intent_alloc` returned with its length, whose
/// first `len` bytes were written.
unsafe fn read<'a>(ptr: *const u8, len: usize) -> &'a [u8] {
    if len == 0 {
        return &[];
    }
    // SAFETY: the caller passes a buffer of intent_alloc.
    unsafe { std::slice::from_raw_parts(ptr, len) }
}

/// intent_alloc allocates a buffer of len bytes.
#[unsafe(no_mangle)]
pub extern "C" fn intent_alloc(len: usize) -> *mut u8 {
    Box::into_raw(vec![0u8; len].into_boxed_slice()) as *mut u8
}

/// intent_free frees a buffer of intent_alloc or of an answer.
///
/// # Safety
/// The buffer must be one this module allocated, with its length.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn intent_free(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    // SAFETY: the buffer was a boxed slice of len bytes.
    drop(unsafe { Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, len)) });
}

/// intent_parse answers a ParseIntentRequest with a ParseIntentResponse.
///
/// # Safety
/// See `read`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn intent_parse(ptr: *const u8, len: usize) -> u64 {
    // SAFETY: as the caller promises.
    match ParseIntentRequest::decode_from_slice(unsafe { read(ptr, len) }) {
        Ok(req) => give(crate::parse(&req).encode_to_vec()),
        Err(_) => 0,
    }
}

/// intent_reply answers a ReplyRequest with a ReplyResponse.
///
/// # Safety
/// See `read`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn intent_reply(ptr: *const u8, len: usize) -> u64 {
    // SAFETY: as the caller promises.
    match ReplyRequest::decode_from_slice(unsafe { read(ptr, len) }) {
        Ok(req) => give(crate::reply(&req).encode_to_vec()),
        Err(_) => 0,
    }
}

/// intent_examples answers an ExamplesRequest with an ExamplesResponse.
///
/// # Safety
/// See `read`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn intent_examples(ptr: *const u8, len: usize) -> u64 {
    // SAFETY: as the caller promises.
    match ExamplesRequest::decode_from_slice(unsafe { read(ptr, len) }) {
        Ok(req) => give(crate::examples(&req).encode_to_vec()),
        Err(_) => 0,
    }
}

/// intent_languages returns a LanguagesResponse.
#[unsafe(no_mangle)]
pub extern "C" fn intent_languages() -> u64 {
    give(crate::languages().encode_to_vec())
}
