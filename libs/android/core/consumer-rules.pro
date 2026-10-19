# The core's Kotlin API calls the core through JNA, which finds the
# library's functions, structures and callbacks by their names.
-keep class com.sun.jna.** { *; }
-keepclassmembers class * extends com.sun.jna.** { *; }
-keep class * implements com.sun.jna.Library { *; }
-keep class * implements com.sun.jna.Callback { *; }
-keep class * extends com.sun.jna.Structure { *; }
-dontwarn java.awt.**
# The server API's messages, whose fields protobuf-lite reaches by reflection.
-keepclassmembers class * extends com.google.protobuf.GeneratedMessageLite { <fields>; }
