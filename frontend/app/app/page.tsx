import type { Metadata } from "next";
import { AppDashboard } from "@/components/app-dashboard";

export const metadata: Metadata = {
  title: "Your files",
  description: "Manage your Aurea Link files, folders, and shared links.",
};

export default function WorkspacePage() {
  return <AppDashboard />;
}
