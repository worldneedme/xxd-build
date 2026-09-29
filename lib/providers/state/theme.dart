part of '../state.dart';

typedef DynamicColorSeeds = ({
  Color? lightSeed,
  Color? darkSeed,
  Color accentColor,
});

@Riverpod(keepAlive: true)
class DynamicColor extends _$DynamicColor {
  @override
  DynamicColorSeeds build() {
    return (
      lightSeed: null,
      darkSeed: null,
      accentColor: const Color(defaultPrimaryColor),
    );
  }

  void seed({Color? lightSeed, Color? darkSeed, required Color accentColor}) {
    state = (
      lightSeed: lightSeed,
      darkSeed: darkSeed,
      accentColor: accentColor,
    );
  }
}

// WONDERX house palettes: keep the seeded accent family but replace every
// tinted surface with neutral tiers, night first, day its cool-white mirror.
ColorScheme _toNeutralSurfaces(ColorScheme scheme, Brightness brightness) {
  if (brightness == Brightness.dark) {
    return scheme.copyWith(
      surface: const Color(0xFF0A0C14),
      surfaceDim: const Color(0xFF08090F),
      surfaceBright: const Color(0xFF20203A),
      surfaceContainerLowest: const Color(0xFF07080E),
      surfaceContainerLow: const Color(0xFF0E1018),
      surfaceContainer: const Color(0xFF151520),
      surfaceContainerHigh: const Color(0xFF1A1A2E),
      surfaceContainerHighest: const Color(0xFF23233A),
      onSurface: const Color(0xFFE8E8EC),
      onSurfaceVariant: const Color(0xFF9CA3AF),
      outline: const Color(0xFF3A3D4D),
      outlineVariant: const Color(0xFF262838),
      surfaceTint: Colors.transparent,
    );
  }
  return scheme.copyWith(
    surface: const Color(0xFFFBFBFC),
    surfaceDim: const Color(0xFFECECF0),
    surfaceBright: const Color(0xFFFFFFFF),
    surfaceContainerLowest: const Color(0xFFFFFFFF),
    surfaceContainerLow: const Color(0xFFF7F7F9),
    surfaceContainer: const Color(0xFFF1F1F4),
    surfaceContainerHigh: const Color(0xFFEBEBEF),
    surfaceContainerHighest: const Color(0xFFE5E5EA),
    onSurface: const Color(0xFF17171A),
    onSurfaceVariant: const Color(0xFF5F5F66),
    outline: const Color(0xFFD3D3D8),
    outlineVariant: const Color(0xFFE4E4E8),
    surfaceTint: Colors.transparent,
  );
}

@riverpod
ColorScheme genColorScheme(
  Ref ref,
  Brightness brightness, {
  Color? color,
  bool ignoreConfig = false,
}) {
  final themeSetting = ref.watch(
    themeSettingProvider.select(
      (state) => (
        primaryColor: state.primaryColor,
        schemeVariant: state.schemeVariant,
      ),
    ),
  );
  final dynamicColor = ref.watch(dynamicColorProvider);
  if (color == null &&
      (ignoreConfig == true || themeSetting.primaryColor == null)) {
    final seed = switch (brightness) {
      Brightness.light => dynamicColor.lightSeed,
      Brightness.dark => dynamicColor.darkSeed,
    };
    return _toNeutralSurfaces(
      ColorScheme.fromSeed(
        seedColor: seed ?? dynamicColor.accentColor,
        brightness: brightness,
        dynamicSchemeVariant: themeSetting.schemeVariant,
      ),
      brightness,
    );
  }
  return _toNeutralSurfaces(
    ColorScheme.fromSeed(
      seedColor: color ?? Color(themeSetting.primaryColor!),
      brightness: brightness,
      dynamicSchemeVariant: themeSetting.schemeVariant,
    ),
    brightness,
  );
}

@riverpod
Brightness currentBrightness(Ref ref) {
  final themeMode = ref.watch(
    themeSettingProvider.select((state) => state.themeMode),
  );
  final systemBrightness = ref.watch(systemBrightnessProvider);
  return switch (themeMode) {
    ThemeMode.system => systemBrightness,
    ThemeMode.light => Brightness.light,
    ThemeMode.dark => Brightness.dark,
  };
}
