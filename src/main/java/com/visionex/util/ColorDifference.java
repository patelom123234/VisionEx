package com.visionex.util;

import java.awt.Color;

public final class ColorDifference {
    private ColorDifference() {}

    public static double ciede2000(Color c1, Color c2) {
        double[] lab1 = rgbToLab(c1);
        double[] lab2 = rgbToLab(c2);
        return ciede2000(lab1, lab2);
    }

    private static double ciede2000(double[] lab1, double[] lab2) {
        double L1 = lab1[0], a1 = lab1[1], b1 = lab1[2];
        double L2 = lab2[0], a2 = lab2[1], b2 = lab2[2];

        double avgLp = (L1 + L2) / 2.0;
        double C1 = Math.hypot(a1, b1);
        double C2 = Math.hypot(a2, b2);
        double avgC = (C1 + C2) / 2.0;

        double G = 0.5 * (1 - Math.sqrt(Math.pow(avgC, 7) / (Math.pow(avgC, 7) + Math.pow(25, 7))));
        double a1p = (1 + G) * a1;
        double a2p = (1 + G) * a2;
        double C1p = Math.hypot(a1p, b1);
        double C2p = Math.hypot(a2p, b2);
        double avgCp = (C1p + C2p) / 2.0;

        double h1p = Math.atan2(b1, a1p);
        h1p = h1p >= 0 ? h1p : h1p + 2 * Math.PI;
        double h2p = Math.atan2(b2, a2p);
        h2p = h2p >= 0 ? h2p : h2p + 2 * Math.PI;

        double deltaLp = L2 - L1;
        double deltaCp = C2p - C1p;

        double deltahp;
        if (C1p * C2p == 0) {
            deltahp = 0;
        } else if (Math.abs(h2p - h1p) <= Math.PI) {
            deltahp = h2p - h1p;
        } else if (h2p <= h1p) {
            deltahp = h2p - h1p + 2 * Math.PI;
        } else {
            deltahp = h2p - h1p - 2 * Math.PI;
        }
        double deltaHp = 2 * Math.sqrt(C1p * C2p) * Math.sin(deltahp / 2.0);

        double avgLpm = (L1 + L2) / 2.0;
        double avgCpm = (C1p + C2p) / 2.0;

        double avghp;
        if (C1p * C2p == 0) {
            avghp = h1p + h2p;
        } else if (Math.abs(h1p - h2p) <= Math.PI) {
            avghp = (h1p + h2p) / 2.0;
        } else if (h1p + h2p < 2 * Math.PI) {
            avghp = (h1p + h2p + 2 * Math.PI) / 2.0;
        } else {
            avghp = (h1p + h2p - 2 * Math.PI) / 2.0;
        }

        double T = 1
                - 0.17 * Math.cos(avghp - Math.toRadians(30))
                + 0.24 * Math.cos(2 * avghp)
                + 0.32 * Math.cos(3 * avghp + Math.toRadians(6))
                - 0.20 * Math.cos(4 * avghp - Math.toRadians(63));

        double deltaTheta = Math.toRadians(30) * Math.exp(-Math.pow((Math.toDegrees(avghp) - 275) / 25.0, 2));
        double Rc = 2 * Math.sqrt(Math.pow(avgCpm, 7) / (Math.pow(avgCpm, 7) + Math.pow(25, 7)));
        double Sl = 1 + (0.015 * Math.pow(avgLpm - 50, 2)) / Math.sqrt(20 + Math.pow(avgLpm - 50, 2));
        double Sc = 1 + 0.045 * avgCpm;
        double Sh = 1 + 0.015 * avgCpm * T;
        double Rt = -Math.sin(2 * deltaTheta) * Rc;

        double dL = deltaLp / Sl;
        double dC = deltaCp / Sc;
        double dH = deltaHp / Sh;
        return Math.sqrt(dL * dL + dC * dC + dH * dH + Rt * dC * dH);
    }

    private static double[] rgbToLab(Color color) {
        double r = pivotRgb(color.getRed() / 255.0);
        double g = pivotRgb(color.getGreen() / 255.0);
        double b = pivotRgb(color.getBlue() / 255.0);

        double x = r * 0.4124 + g * 0.3576 + b * 0.1805;
        double y = r * 0.2126 + g * 0.7152 + b * 0.0722;
        double z = r * 0.0193 + g * 0.1192 + b * 0.9505;

        return xyzToLab(x, y, z);
    }

    private static double pivotRgb(double value) {
        return value > 0.04045 ? Math.pow((value + 0.055) / 1.055, 2.4) : value / 12.92;
    }

    private static double[] xyzToLab(double x, double y, double z) {
        double refX = 0.95047;
        double refY = 1.00000;
        double refZ = 1.08883;

        double fx = pivotXyz(x / refX);
        double fy = pivotXyz(y / refY);
        double fz = pivotXyz(z / refZ);

        double L = Math.max(0, 116 * fy - 16);
        double a = 500 * (fx - fy);
        double b = 200 * (fy - fz);
        return new double[]{L, a, b};
    }

    private static double pivotXyz(double value) {
        return value > 0.008856 ? Math.cbrt(value) : (7.787 * value) + 16.0 / 116.0;
    }
}

