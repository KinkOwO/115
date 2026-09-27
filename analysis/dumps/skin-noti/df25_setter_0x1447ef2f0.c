// setter_0x1447ef2f0

__int64 __fastcall sub_1447EF2F0(__int64 a1, int a2)
{
  __int64 result; // rax
  __int64 v5; // rax
  unsigned __int8 v6; // al
  __int64 v7; // rcx
  int v8; // [rsp+38h] [rbp+10h] BYREF

  *(_DWORD *)(a1 + 112) = a2;
  result = sub_145EFAFB0();
  if ( result )
  {
    v5 = sub_145EFAFB0();
    v8 = a2;
    sub_145D83900(v5, &v8);
    (*(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)qword_14E683C68 + 16LL))(qword_14E683C68, 178, 2);
    v6 = sub_145F12D90(qword_14E683C20);
    sub_1466642E0(qword_14E683C68, v6);
    sub_146664340(qword_14E683C68, *(unsigned int *)(a1 + 112));
    LOBYTE(v7) = 1;
    return sub_14665AA60(v7, 0);
  }
  return result;
}

