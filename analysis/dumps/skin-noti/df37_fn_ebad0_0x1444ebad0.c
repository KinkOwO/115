// fn_ebad0_0x1444ebad0

__int64 __fastcall sub_1444EBAD0(__int64 a1)
{
  unsigned int v1; // ebx
  __int64 v2; // rax
  __int64 v3; // rdi
  __int64 v4; // rcx
  void (__fastcall ***v5)(_QWORD); // rax

  v1 = 100000;
  v2 = *(_QWORD *)(a1 + 1152);
  v3 = *(_QWORD *)(a1 + 1160);
  if ( v2 != v3 )
    v1 = *(_DWORD *)(*(_QWORD *)(a1 + 1152) + 4LL * (int)((int)sub_146E9BA90(a1) % (unsigned __int64)((v3 - v2) >> 2)));
  v4 = qword_14E634518;
  if ( !qword_14E634518 )
  {
    v5 = (void (__fastcall ***)(_QWORD))sub_146E8BA20(2008);
    if ( v5 )
      v5 = (void (__fastcall ***)(_QWORD))sub_143C9DEC0(v5);
    qword_14E634518 = (__int64)v5;
    (**v5)(v5);
    v4 = qword_14E634518;
  }
  if ( (unsigned __int8)sub_143CA0E80(v4) )
    return 100000;
  return v1;
}

