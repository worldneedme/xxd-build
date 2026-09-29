import 'package:fl_clash/common/constant.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:material_ui/material_ui.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'DynamicColor falls back to the default primary color before seeding',
    () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(
        container.read(dynamicColorProvider).accentColor,
        const Color(defaultPrimaryColor),
      );
    },
  );

  test('genColorScheme seeds each brightness from its own dynamic seed', () {
    const lightSeed = Color(0xFF00FF00);
    const darkSeed = Color(0xFF0000FF);
    final container = ProviderContainer();
    addTearDown(container.dispose);
    container
        .read(dynamicColorProvider.notifier)
        .seed(
          lightSeed: lightSeed,
          darkSeed: darkSeed,
          accentColor: const Color(defaultPrimaryColor),
        );

    final variant = container.read(themeSettingProvider).schemeVariant;

    // Accents follow the seed; surfaces stay on the WONDERX house tiers.
    final light = container.read(genColorSchemeProvider(Brightness.light));
    expect(
      light.primary,
      ColorScheme.fromSeed(
        seedColor: lightSeed,
        brightness: Brightness.light,
        dynamicSchemeVariant: variant,
      ).primary,
    );
    expect(light.surface, const Color(0xFFFBFBFC));

    final dark = container.read(genColorSchemeProvider(Brightness.dark));
    expect(
      dark.primary,
      ColorScheme.fromSeed(
        seedColor: darkSeed,
        brightness: Brightness.dark,
        dynamicSchemeVariant: variant,
      ).primary,
    );
    expect(dark.surface, const Color(0xFF0A0C14));
  });

  test('genColorScheme falls back to the accent color without a seed', () {
    const accent = Color(0xFFFF0000);
    final container = ProviderContainer();
    addTearDown(container.dispose);
    container
        .read(dynamicColorProvider.notifier)
        .seed(lightSeed: null, darkSeed: null, accentColor: accent);

    final scheme = container.read(genColorSchemeProvider(Brightness.light));
    expect(
      scheme.primary,
      ColorScheme.fromSeed(
        seedColor: accent,
        brightness: Brightness.light,
        dynamicSchemeVariant: container
            .read(themeSettingProvider)
            .schemeVariant,
      ).primary,
    );
    expect(scheme.surface, const Color(0xFFFBFBFC));
  });
}
