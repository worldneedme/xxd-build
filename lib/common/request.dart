import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:dio/io.dart';
import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/enum/enum.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';

class Request {
  late final Dio dio;
  late final Dio _clashDio;
  late final Dio _directDio;
  String? userAgent;

  ProviderReader? _read;

  void attach(ProviderReader read) {
    _read = read;
  }

  Request() {
    dio = Dio(BaseOptions(
      headers: {'User-Agent': browserUa},
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
    ));
    _clashDio = Dio();
    _clashDio.httpClientAdapter = IOHttpClientAdapter(
      createHttpClient: () {
        final client = HttpClient();
        client.findProxy = (Uri uri) {
          client.userAgent = globalState.ua;
          final read = _read;
          if (read == null) {
            return 'DIRECT';
          }
          return FlClashHttpOverrides.findProxyForReader(read, uri);
        };
        return client;
      },
    );
    // A subscription URL must still be reachable when the currently selected
    // proxy is stale. This client bypasses the global FlClash HttpOverrides.
    _directDio = Dio(BaseOptions(
      headers: {'User-Agent': browserUa},
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
    ));
    _directDio.httpClientAdapter = IOHttpClientAdapter(
      createHttpClient: () {
        final client = HttpClient();
        client.findProxy = (_) => 'DIRECT';
        return client;
      },
    );
  }

  Future<Response<Uint8List>> getFileResponseForUrl(String url) async {
    final targetUrl = _normalizeUrl(url);
    try {
      return await _clashDio.get<Uint8List>(
        targetUrl,
        options: Options(responseType: ResponseType.bytes),
      );
    } catch (e) {
      commonPrint.log(
        'getFileResponseForUrl proxy error type=${e is DioException ? e.type : e.runtimeType} ${e.toString()}',
      );
      if (e is DioException) {
        if (e.type == DioExceptionType.cancel) {
          rethrow;
        }
      }

      // The first attempt follows the active proxy. Retry directly because
      // subscription delivery is independent of the selected node and this
      // also handles a dead/stale mixed-port connection.
      try {
        final response = await _directDio.get<Uint8List>(
          targetUrl,
          options: Options(responseType: ResponseType.bytes),
        );
        commonPrint.log('getFileResponseForUrl direct retry succeeded');
        return response;
      } catch (directError) {
        commonPrint.log(
          'getFileResponseForUrl direct error type=${directError is DioException ? directError.type : directError.runtimeType} ${directError.toString()}',
        );
        if (directError is DioException) {
          if (directError.type == DioExceptionType.badResponse) {
            throw MessageException(_formatBadResponse(directError));
          }
          if (directError.type == DioExceptionType.cancel) {
            rethrow;
          }
          throw MessageException(_formatNetworkError(directError));
        }
      }
      throw MessageException(currentAppLocalizations.unknownNetworkError);
    }
  }

  String _normalizeUrl(String url) {
    var value = url.trim();
    if (value.length >= 2 &&
        ((value.startsWith('"') && value.endsWith('"')) ||
            (value.startsWith("'") && value.endsWith("'")))) {
      value = value.substring(1, value.length - 1).trim();
    }
    return value;
  }

  String _formatNetworkError(DioException exception) {
    final detail = exception.message?.trim();
    switch (exception.type) {
      case DioExceptionType.connectionTimeout:
        return '订阅连接超时';
      case DioExceptionType.sendTimeout:
        return '订阅发送超时';
      case DioExceptionType.receiveTimeout:
        return '订阅接收超时';
      case DioExceptionType.badCertificate:
        return '订阅 TLS 证书错误';
      case DioExceptionType.connectionError:
        return detail == null || detail.isEmpty
            ? '订阅连接失败'
            : '订阅连接失败: $detail';
      default:
        return detail == null || detail.isEmpty
            ? currentAppLocalizations.unknownNetworkError
            : detail;
    }
  }

  String _formatBadResponse(DioException exception) {
    final response = exception.response;
    final statusCode = response?.statusCode;
    final statusMessage = response?.statusMessage?.trim();
    final serverMessage = _extractResponseMessage(response?.data);
    final statusLabel = statusCode != null
        ? 'HTTP $statusCode'
        : currentAppLocalizations.networkException;
    if (serverMessage.isNotEmpty) {
      return '$statusLabel: $serverMessage';
    }
    if (statusMessage != null && statusMessage.isNotEmpty) {
      return '$statusLabel $statusMessage';
    }
    return statusLabel;
  }

  String _extractResponseMessage(Object? data) {
    final body = _responseBodyToString(data).trim();
    if (body.isEmpty) return '';
    try {
      final decoded = json.decode(body);
      if (decoded is Map) {
        for (final key in ['message', 'error', 'detail', 'msg']) {
          final value = decoded[key];
          if (value is String && value.trim().isNotEmpty) {
            return value.trim();
          }
        }
      }
    } catch (_) {
      // Non-JSON error pages are still useful when they are short enough.
    }
    return body.replaceAll(RegExp(r'\s+'), ' ').safeSubstring(0, 300);
  }

  String _responseBodyToString(Object? data) {
    if (data == null) return '';
    if (data is String) return data;
    if (data is List<int>) {
      return utf8.decode(data, allowMalformed: true);
    }
    return data.toString();
  }

  Future<Response<String>> getTextResponseForUrl(String url) async {
    try {
      return await _clashDio.get<String>(
        url,
        options: Options(responseType: ResponseType.plain),
      );
    } catch (e) {
      commonPrint.log(
        'getTextResponseForUrl error ${compactError(e)}',
        logLevel: LogLevel.warning,
      );
      rethrow;
    }
  }

  Future<Map<String, dynamic>?> checkForUpdate() async {
    try {
      final response = await dio.get(
        'https://api.github.com/repos/$repository/releases/latest',
        options: Options(responseType: ResponseType.json),
      );
      if (response.statusCode != 200) return null;
      final data = response.data as Map<String, dynamic>;
      final remoteVersion = data['tag_name'];
      final version = globalState.packageInfo.version;
      final hasUpdate =
          compareVersions(remoteVersion.replaceAll('v', ''), version) > 0;
      if (!hasUpdate) return null;
      return data;
    } catch (e) {
      commonPrint.log('checkForUpdate failed', logLevel: LogLevel.warning);
      return null;
    }
  }

  final Map<String, IpInfo Function(Map<String, dynamic>)> _ipInfoSources = {
    'https://ipwho.is': IpInfo.fromIpWhoIsJson,
    'https://api.myip.com': IpInfo.fromMyIpJson,
    'https://ipapi.co/json': IpInfo.fromIpApiCoJson,
    'https://ident.me/json': IpInfo.fromIdentMeJson,
    'http://ip-api.com/json': IpInfo.fromIpAPIJson,
    'https://api.ip.sb/geoip': IpInfo.fromIpSbJson,
    'https://ipinfo.io/json': IpInfo.fromIpInfoIoJson,
  };

  Future<Result<IpInfo?>> checkIp({CancelToken? cancelToken}) async {
    final token = cancelToken ?? CancelToken();
    final isStart = globalState.container.read(isStartProvider);
    final client = isStart ? _clashDio : dio;
    for (final source in _ipInfoSources.entries) {
      if (token.isCancelled) {
        return Result.error('cancelled');
      }
      try {
        final res = await client
            .get<Map<String, dynamic>>(
              source.key,
              cancelToken: token,
              options: Options(responseType: ResponseType.json),
            )
            .timeout(const Duration(seconds: 8));
        if (res.statusCode == HttpStatus.ok && res.data != null) {
          return Result.success(source.value(res.data!));
        }
        commonPrint.log(
          'checkIp ${source.key} data empty',
          logLevel: LogLevel.info,
        );
      } catch (e) {
        if (e is DioException && e.type == DioExceptionType.cancel) {
          return Result.error('cancelled');
        }
        commonPrint.log(
          'checkIp ${source.key} error $e',
          logLevel: LogLevel.warning,
        );
      }
    }
    return Result.success(null);
  }
}

final request = Request();

String? getFileNameForDisposition(String? disposition) {
  if (disposition == null) return null;
  final parseValue = HeaderValue.parse(disposition);
  final parameters = parseValue.parameters;
  final fileNamePointKey = parameters.keys.firstWhere(
    (key) => key == 'filename*',
    orElse: () => '',
  );
  if (fileNamePointKey.isNotEmpty) {
    final res = parameters[fileNamePointKey]?.split("''") ?? [];
    if (res.length >= 2) {
      return Uri.decodeComponent(res[1]);
    }
  }
  final fileNameKey = parameters.keys.firstWhere(
    (key) => key == 'filename',
    orElse: () => '',
  );
  if (fileNameKey.isEmpty) return null;
  return parameters[fileNameKey];
}
