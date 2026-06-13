namespace go userprofile

struct Profile {
  1: optional string account_status
}

struct UserProfile {
  1: required i32 register_days
  2: optional string kyc_level
  3: optional Profile profile
}

service UserProfileService {
  UserProfile GetUserProfile(1: required string user_id)
}
