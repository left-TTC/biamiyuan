#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""批量将文件夹中的 SVG 文件转换为 PNG 图片。

依赖(二选一, 程序会自动检测可用引擎):
    1. cairosvg          ->  pip install cairosvg
    2. svglib+reportlab  ->  pip install svglib reportlab

用法示例:
    python svg2png.py                 # 转换脚本所在目录中的 SVG
    python svg2png.py ./icons         # 转换指定文件夹
    python svg2png.py ./icons -o png  # 输出到 png 目录
    python svg2png.py ./icons -s 512  # 输出 512x512(保持宽高比)
    python svg2png.py ./icons --scale 2   # 整体放大 2 倍
    python svg2png.py ./assets -r     # 递归转换所有子目录
    python svg2png.py ./icons -w 200 -H 200  # 指定宽高
"""

import argparse
import os
import subprocess
import sys


def find_homebrew_cairo_dir():
    """在 macOS 上定位 Homebrew 安装的 cairo 原生库目录。"""
    if sys.platform != "darwin":
        return None
    candidates = []
    # Apple Silicon (/opt/homebrew) 与 Intel (/usr/local) 两种安装位置
    for prefix in ("/opt/homebrew", "/usr/local"):
        lib_dir = os.path.join(prefix, "opt", "cairo", "lib")
        if os.path.isfile(os.path.join(lib_dir, "libcairo.2.dylib")):
            candidates.append(lib_dir)
    # 兜底: 用 `brew --prefix cairo` 查询
    try:
        prefix = subprocess.check_output(
            ["brew", "--prefix", "cairo"], text=True
        ).strip()
        lib_dir = os.path.join(prefix, "lib")
        if os.path.isfile(os.path.join(lib_dir, "libcairo.2.dylib")):
            candidates.append(lib_dir)
    except (OSError, subprocess.SubprocessError):
        pass
    return candidates[0] if candidates else None


def ensure_cairo_on_path():
    """让 cairocffi(ctypes) 能找到 Homebrew 的 libcairo。

    macOS 上 cairocffi 通过 ctypes.util.find_library() 查找 libcairo,
    该函数在调用时会读取 DYLD_LIBRARY_PATH 环境变量, 因此只要在
    import cairosvg 之前把 cairo 库目录写进环境变量即可自动生效,
    无需用户手动设置。
    """
    cairo_dir = find_homebrew_cairo_dir()
    if not cairo_dir:
        return
    for var in ("DYLD_LIBRARY_PATH", "DYLD_FALLBACK_LIBRARY_PATH"):
        current = os.environ.get(var, "")
        if cairo_dir not in current.split(":"):
            os.environ[var] = ":".join(
                part for part in (cairo_dir, current) if part
            )


# 先确保 libcairo 可被找到, 再导入依赖
# (缺失时静默跳过, 由 pick_engine 给出具体原因与安装提示)
ensure_cairo_on_path()

try:
    import cairosvg  # noqa: F401
    HAS_CAIROSVG = True
    CAIROSVG_IMPORT_ERROR = None
except Exception as exc:  # noqa: BLE001  (cairocffi 底层缺 cairo 库时也会抛错)
    HAS_CAIROSVG = False
    CAIROSVG_IMPORT_ERROR = exc

try:
    import svglib  # noqa: F401
    import reportlab  # noqa: F401
    HAS_SVGLIB = True
    SVGLIB_IMPORT_ERROR = None
except Exception as exc:  # noqa: BLE001
    HAS_SVGLIB = False
    SVGLIB_IMPORT_ERROR = exc


def die(message):
    """向 stderr 输出错误信息并退出。"""
    print(message, file=sys.stderr)
    sys.exit(1)


def _err(exc):
    """取异常信息的首行, 便于紧凑地展示导入失败原因。"""
    return str(exc).splitlines()[0] if exc else "未安装"


def pick_engine(engine):
    """根据用户指定或自动检测选择一个可用的转换引擎。"""
    if engine == "cairosvg":
        if HAS_CAIROSVG:
            return "cairosvg"
        die("cairosvg 不可用: %s\n"
            "请安装: pip install cairosvg%s"
            % (_err(CAIROSVG_IMPORT_ERROR), _cairo_hint()))
    if engine == "svglib":
        if HAS_SVGLIB:
            return "svglib"
        die("svglib/reportlab 不可用: %s\n"
            "请安装: pip install svglib reportlab"
            % _err(SVGLIB_IMPORT_ERROR))
    # auto
    if HAS_CAIROSVG:
        return "cairosvg"
    if HAS_SVGLIB:
        return "svglib"
    die(_no_engine_message())


def _cairo_hint():
    """针对 macOS 缺 libcairo 给出补充提示。"""
    detail = str(CAIROSVG_IMPORT_ERROR or "").lower()
    if sys.platform == "darwin" and "cairo" in detail:
        return "\nmacOS 上还需安装 cairo 原生库: brew install cairo"
    return ""


def _no_engine_message():
    """汇总两个引擎均不可用时的报错信息。"""
    lines = [
        "未找到可用的转换库, 请安装其中之一:",
        "    pip install cairosvg",
        "或:",
        "    pip install svglib reportlab",
    ]
    if CAIROSVG_IMPORT_ERROR:
        lines.append("")
        lines.append("cairosvg 导入失败原因: %s" % _err(CAIROSVG_IMPORT_ERROR))
        hint = _cairo_hint()
        if hint:
            lines.append(hint.lstrip("\n"))
    return "\n".join(lines)


def collect_svg_files(root, recursive):
    """收集根目录下的所有 .svg 文件路径。"""
    if recursive:
        for dirpath, _dirnames, filenames in os.walk(root):
            for name in sorted(filenames):
                if name.lower().endswith(".svg"):
                    yield os.path.join(dirpath, name)
    else:
        for name in sorted(os.listdir(root)):
            if name.lower().endswith(".svg"):
                yield os.path.join(root, name)


def convert_cairosvg(svg_path, png_path, width, height, scale):
    """使用 cairosvg 渲染。"""
    kwargs = {}
    if width:
        kwargs["output_width"] = width
    if height:
        kwargs["output_height"] = height
    if scale:
        kwargs["scale"] = scale
    cairosvg.svg2png(url=svg_path, write_to=png_path, **kwargs)


def convert_svglib(svg_path, png_path, width, height, scale):
    """使用 svglib + reportlab 渲染(备用引擎)。"""
    from svglib.svglib import svg2rlg
    from reportlab.graphics import renderPM

    drawing = svg2rlg(svg_path)
    if drawing is None:
        raise RuntimeError("svglib 无法解析该 SVG")

    if width or height or scale:
        base_w = drawing.width or 1.0
        base_h = drawing.height or 1.0
        if width and height:
            target_w, target_h = float(width), float(height)
        elif width:
            target_w = float(width)
            target_h = target_w * base_h / base_w
        elif height:
            target_h = float(height)
            target_w = target_h * base_w / base_h
        else:
            target_w, target_h = base_w, base_h
        if scale:
            target_w *= scale
            target_h *= scale
        drawing.width = target_w
        drawing.height = target_h
        drawing.scale(target_w / base_w, target_h / base_h)

    renderPM.drawToFile(drawing, png_path, fmt="PNG")


def main(argv=None):
    parser = argparse.ArgumentParser(
        prog="svg2png.py",
        description="批量将文件夹中的 SVG 文件转换为 PNG 图片。",
        epilog="依赖: pip install cairosvg  或  pip install svglib reportlab",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "input", nargs="?", default=None,
        help="包含 SVG 的文件夹, 默认是脚本所在目录",
    )
    parser.add_argument(
        "-o", "--output", default=None,
        help="PNG 输出目录, 默认与输入目录相同",
    )
    parser.add_argument(
        "-w", "--width", type=int, default=None,
        help="输出宽度(像素); 未指定高度时自动保持宽高比",
    )
    parser.add_argument(
        "-H", "--height", type=int, default=None,
        help="输出高度(像素); 未指定宽度时自动保持宽高比",
    )
    parser.add_argument(
        "-s", "--size", type=int, default=None,
        help="输出边长(像素), 等价于同时指定宽和高",
    )
    parser.add_argument(
        "--scale", type=float, default=None,
        help="整体缩放倍数, 例如 2 表示放大两倍",
    )
    parser.add_argument(
        "-r", "--recursive", action="store_true",
        help="递归处理输入目录下的所有子目录",
    )
    parser.add_argument(
        "--engine", choices=["auto", "cairosvg", "svglib"], default="auto",
        help="使用的转换引擎, 默认 auto 自动选择",
    )
    args = parser.parse_args(argv)

    # 尺寸参数: -s 同时覆盖宽高
    if args.size is not None:
        width = height = args.size
    else:
        width, height = args.width, args.height

    input_dir = os.path.abspath(args.input) if args.input else \
        os.path.dirname(os.path.abspath(__file__))
    if not os.path.isdir(input_dir):
        parser.error("输入文件夹不存在: %s" % input_dir)

    output_dir = os.path.abspath(args.output) if args.output else input_dir
    os.makedirs(output_dir, exist_ok=True)

    engine = pick_engine(args.engine)
    print("使用引擎: %s" % engine)

    svg_files = sorted(collect_svg_files(input_dir, args.recursive))
    if not svg_files:
        print("在 %s 中未找到 SVG 文件。" % input_dir)
        print("提示: 若文件在子目录中, 请加 -r 递归处理, 或直接指定文件夹路径。")
        return 0

    ok = fail = 0
    for svg_path in svg_files:
        rel = os.path.relpath(svg_path, input_dir)
        png_rel = os.path.splitext(rel)[0] + ".png"
        png_path = os.path.join(output_dir, png_rel)
        os.makedirs(os.path.dirname(png_path), exist_ok=True)

        try:
            if engine == "cairosvg":
                convert_cairosvg(svg_path, png_path, width, height, args.scale)
            else:
                convert_svglib(svg_path, png_path, width, height, args.scale)
        except Exception as exc:  # noqa: BLE001
            fail += 1
            print("失败: %s -> %s  (%s)" % (rel, png_rel, exc))
        else:
            ok += 1
            print("已转换: %s -> %s" % (rel, png_rel))

    print("完成: 成功 %d 个, 失败 %d 个, 输出目录: %s" % (ok, fail, output_dir))
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main())


