import 'dart:ui' show ImageFilter;

import 'package:fl_clash/common/common.dart';
import 'package:material_ui/material_ui.dart';

// WONDERX aurora backdrop + liquid glass. Blur only sells when there is colour
// behind it, so the glows are drawn from the active scheme and every glass
// panel sits on top of them.
class AuroraBackground extends StatelessWidget {
  const AuroraBackground({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final colorScheme = context.colorScheme;
    final isDark = colorScheme.brightness == Brightness.dark;
    return ColoredBox(
      color: colorScheme.surface,
      child: RepaintBoundary(
        child: Stack(
          fit: StackFit.expand,
          children: [
            Positioned(
              top: -120,
              left: -180,
              child: _Glow(
                color: colorScheme.primary,
                size: 540,
                alpha: isDark ? 0.24 : 0.20,
              ),
            ),
            Positioned(
              right: -140,
              bottom: -160,
              child: _Glow(
                color: colorScheme.tertiary,
                size: 460,
                alpha: isDark ? 0.18 : 0.14,
              ),
            ),
            Positioned(
              left: 60,
              bottom: -120,
              child: _Glow(
                color: colorScheme.secondary,
                size: 320,
                alpha: isDark ? 0.16 : 0.14,
              ),
            ),
            child,
          ],
        ),
      ),
    );
  }
}

class _Glow extends StatelessWidget {
  const _Glow({required this.color, required this.size, required this.alpha});

  final Color color;
  final double size;
  final double alpha;

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(
          gradient: RadialGradient(
            colors: [
              color.withValues(alpha: alpha),
              color.withValues(alpha: 0),
            ],
          ),
        ),
      ),
    );
  }
}

class GlassPanel extends StatelessWidget {
  const GlassPanel({
    super.key,
    this.radius = AppCorner.lg,
    this.blurSigma = 20,
    this.margin = EdgeInsets.zero,
    required this.child,
  });

  final double radius;
  final double blurSigma;
  final EdgeInsetsGeometry margin;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final colorScheme = context.colorScheme;
    final isDark = colorScheme.brightness == Brightness.dark;
    final borderRadius = AppRadius.all(radius);
    final body = Stack(
      children: [
        Positioned.fill(
          child: ColoredBox(
            color: (isDark ? colorScheme.surfaceContainerHigh : Colors.white)
                .withValues(alpha: isDark ? 0.52 : 0.72),
          ),
        ),
        // Specular rim + top sheen, or the panel reads as fog instead of glass.
        Positioned.fill(
          child: IgnorePointer(
            child: DecoratedBox(
              decoration: ShapeDecoration(
                shape: RoundedSuperellipseBorder(
                  borderRadius: borderRadius,
                  side: BorderSide(
                    color: isDark
                        ? Colors.white.withValues(alpha: 0.14)
                        : Colors.black.withValues(alpha: 0.07),
                    width: hairline,
                  ),
                ),
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  stops: const [0.0, 0.35],
                  colors: [
                    Colors.white.withValues(alpha: isDark ? 0.10 : 0.40),
                    Colors.white.withValues(alpha: 0),
                  ],
                ),
              ),
            ),
          ),
        ),
        child,
      ],
    );
    return Padding(
      padding: margin,
      child: DecoratedBox(
        decoration: ShapeDecoration(
          shape: RoundedSuperellipseBorder(borderRadius: borderRadius),
          shadows: [
            BoxShadow(
              color: Colors.black.withValues(alpha: isDark ? 0.36 : 0.07),
              blurRadius: isDark ? 28.0 : 24.0,
              offset: Offset(0, isDark ? 10.0 : 6.0),
            ),
            if (!isDark)
              BoxShadow(
                color: Colors.black.withValues(alpha: 0.04),
                blurRadius: 2,
                offset: const Offset(0, 1),
              ),
          ],
        ),
        child: ClipRSuperellipse(
          borderRadius: borderRadius,
          child: BackdropFilter(
            filter: ImageFilter.blur(sigmaX: blurSigma, sigmaY: blurSigma),
            child: body,
          ),
        ),
      ),
    );
  }
}
