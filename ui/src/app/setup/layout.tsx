export default function SetupLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-gradient-to-b from-sercha-snow to-sercha-mist">
      {children}
    </div>
  );
}
