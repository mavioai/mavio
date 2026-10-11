//! Round trips of generated messages through the protobuf encoding.

use buffa::Message;
use mavio_api::proto::mavio::auth::v1::Device;

#[test]
fn round_trip() {
    let device = Device {
        id: Some("d1".into()),
        name: Some("Living room".into()),
        ..Default::default()
    };
    let got = Device::decode_from_slice(&device.encode_to_vec()).unwrap();
    assert_eq!(got, device, "got = {got:?}, want = {device:?}");
}
